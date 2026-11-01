package mc

import (
	"time"

	"cairn/internal/nbt"
)

func (s *Session) handleExtraEvent(id byte, pkt []byte) bool {
	switch id {
	case IDCommandOutput:
		co, err := ParseCommandOutput(pkt)
		if err != nil {
			s.log("⚠ CommandOutput 解析失败：%v", err)
			return true
		}
		txt := co.Text()
		s.log("← 指令结果 count=%d: %s", co.SuccessCount, txt)
		s.emitEvent("command_output", map[string]any{
			"origin":        co.OriginType,
			"request_id":    co.RequestID,
			"output_type":   co.OutputType,
			"success_count": co.SuccessCount,
			"text":          txt,
			"messages":      co.Messages,
		})
		return true

	case IDContainerOpen:
		ci, err := ParseContainerOpen(pkt)
		if err != nil {
			return true
		}

		s.containerMu.Lock()
		s.containerOpenedAt = time.Now()
		s.containerWindow = ci.WindowID
		s.containerType = ci.WindowType
		s.containerMu.Unlock()

		s.log("← 容器打开 windowID=%d type=%d pos=(%d,%d,%d)",
			ci.WindowID, ci.WindowType, ci.Pos.X, ci.Pos.Y, ci.Pos.Z)
		s.emitEvent("container_open", map[string]any{
			"window_id":   ci.WindowID,
			"window_type": ci.WindowType,
			"x":           ci.Pos.X, "y": ci.Pos.Y, "z": ci.Pos.Z,
			"entity_id": ci.EntityID,
		})
		return true

	case IDRespawn:
		ri, err := ParseRespawn(pkt)
		if err != nil {
			return true
		}
		s.log("← 重生 state=%d pos=(%.1f,%.1f,%.1f)", ri.State, ri.X, ri.Y, ri.Z)
		s.emitEvent("respawn", map[string]any{
			"state": ri.State, "x": ri.X, "y": ri.Y, "z": ri.Z,
			"entity_id": ri.EntityID,
		})
		return true

	case IDSetHealth:
		hi, err := ParseSetHealth(pkt)
		if err != nil {
			return true
		}
		s.emitEvent("health", map[string]any{
			"health": hi.Health, "max_health": hi.MaxHealth,
		})
		return true

	case IDDeathInfo:
		di, err := ParseDeathInfo(pkt)
		if err != nil {
			return true
		}
		s.log("← 死亡信息：%s", di.Cause)
		s.emitEvent("death", map[string]any{
			"cause": di.Cause, "messages": di.Messages,
		})
		return true

	case IDInventoryContent:

		ic, err := ParseInventoryContent(pkt)
		if err != nil {
			s.log("⚠ InventoryContent 解析失败：%v", err)
			return true
		}
		n := ic.NonAirCount()
		s.log("← 容器内容 windowID=%d 槽位=%d 非空=%d", ic.WindowID, len(ic.Slots), n)

		var items []map[string]any
		for i, sl := range ic.Slots {
			if sl.IsAir() {
				continue
			}
			items = append(items, map[string]any{
				"slot": i, "network_id": sl.NetworkID, "count": sl.Count,
				"metadata": sl.Metadata, "block_runtime_id": sl.BlockRuntimeID,
			})
		}
		s.emitEvent("inventory_content", map[string]any{
			"window_id":  ic.WindowID,
			"slots":      len(ic.Slots),
			"item_count": n,
			"items":      items,
		})
		return true

	case IDSubChunk:

		sum, err := DecodeSubChunkSummary(pkt[1:])
		if err != nil {
			s.log("⚠ SubChunk 解析失败：%v", err)
			return true
		}
		s.log("← 子区块 version=%d yIndex=%v 调色板=%d 非空气=%d",
			sum.Version, sum.HasYIndex, sum.PaletteSize, sum.Total)

		shown := 0
		for name, n := range sum.ByBlock {
			if shown >= 4 {
				break
			}
			s.log("     %s × %d", name, n)
			shown++
		}

		s.emitEvent("subchunk", map[string]any{
			"version":      sum.Version,
			"has_y_index":  sum.HasYIndex,
			"y_index":      sum.YIndex,
			"palette_size": sum.PaletteSize,
			"block_count":  sum.Total,
			"by_block":     sum.ByBlock,
		})
		return true

	case IDBlockActorData:

		pos, off, err := rdBlockPos(pkt, 1)
		if err != nil {
			s.log("⚠ BlockActorData 坐标解析失败：%v", err)
			return true
		}
		if off >= len(pkt) {
			s.log("← 方块实体 pos=(%d,%d,%d) 但没有 NBT 载荷", pos.X, pos.Y, pos.Z)
			return true
		}

		nbtName, c, used, nerr := nbt.ParseNetwork(pkt[off:])
		if nerr != nil {
			s.log("⚠ 方块实体 NBT 解析失败 pos=(%d,%d,%d)：%v", pos.X, pos.Y, pos.Z, nerr)
			s.emitEvent("block_actor_data", map[string]any{
				"x": pos.X, "y": pos.Y, "z": pos.Z,
				"raw_len": len(pkt) - off, "error": nerr.Error(),
			})
			return true
		}

		id := c.GetStr("id", "")
		n := 0
		if items := c.GetList("Items"); items != nil {
			n = len(items)
		}

		neTag := SplitNETag(c)
		if neTag != nil {
			_ = UnpackNETag(neTag)
			DumpNETagSamples(neTag)
			s.log("← 方块实体 id=%s pos=(%d,%d,%d) __tag=%d字节（已存样本）",
				neTag.ID, pos.X, pos.Y, pos.Z, len(neTag.Raw))
		}
		s.log("← 方块实体 %s pos=(%d,%d,%d) 字段=%d 物品=%d nbt用了%d/%d字节",
			id, pos.X, pos.Y, pos.Z, len(c), n, used, len(pkt)-off)

		ev := map[string]any{
			"x": pos.X, "y": pos.Y, "z": pos.Z,
			"id":         id,
			"root_name":  nbtName,
			"item_count": n,
			"nbt":        nbtToAny(c),
		}
		if neTag != nil {
			ev["netease_tag"] = neTag.Data
		}
		s.emitEvent("block_actor_data", ev)
		return true
	}
	return false
}

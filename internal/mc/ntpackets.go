package mc

const (
	ActionStartBreak         = 0
	ActionAbortBreak         = 1
	ActionStopBreak          = 2
	ActionGetUpdatedBlock    = 3
	ActionDropItem           = 4
	ActionStartSleeping      = 5
	ActionStopSleeping       = 6
	ActionRespawn            = 7
	ActionJump               = 8
	ActionStartSprint        = 9
	ActionStopSprint         = 10
	ActionStartSneak         = 11
	ActionStopSneak          = 12
	ActionCreativeDestroy    = 13
	ActionDimensionChange    = 14
	ActionStartGlide         = 15
	ActionStopGlide          = 16
	ActionBuildDenied        = 17
	ActionCrackBreak         = 18
	ActionChangeSkin         = 19
	ActionSetEnchantmentSeed = 20
	ActionStartSwimming      = 21
	ActionStopSwimming       = 22
	ActionStartSpinAttack    = 23
	ActionStopSpinAttack     = 24
	ActionInteractBlock      = 25
	ActionPredictBreak       = 26
	ActionContinueBreak      = 27
)

const (
	InteractLeaveVehicle    = 0
	InteractMouseOverEntity = 1
	InteractNpcOpen         = 2
	InteractOpenInventory   = 3
)

func BuildInteractOpenInventory(runtimeID int64) []byte {
	b := WriteVarint(NTIDInteract)
	b = append(b, WriteVarint(InteractOpenInventory)...)
	b = append(b, WriteVarint64(uint64(runtimeID))...)
	return b
}

func BuildPlayerAction(runtimeID uint64, actionType int, bx, by, bz, rx, ry, rz, face int32) []byte {
	b := WriteVarint(NTIDPlayerAction)
	b = append(b, WriteVarint64(runtimeID)...)
	b = append(b, WriteVarint(uint32(actionType))...)
	b = append(b, WriteVarint(uint32(bx))...)
	b = append(b, WriteVarint(uint32(by))...)
	b = append(b, WriteVarint(uint32(bz))...)
	b = append(b, WriteVarint(uint32(rx))...)
	b = append(b, WriteVarint(uint32(ry))...)
	b = append(b, WriteVarint(uint32(rz))...)
	b = append(b, WriteVarint(uint32(face))...)
	return b
}

const (
	MoveModeNormal   = 0
	MoveModeReset    = 1
	MoveModeTeleport = 2
	MoveModeRotation = 3
)

func BuildMovePlayer(runtimeID int64, x, y, z, pitch, yaw, headYaw float32, mode byte, onGround bool, riddenID int64, tick int64) []byte {
	b := WriteVarint(NTIDMovePlayer)
	b = append(b, WriteZigzag64(uint64(runtimeID))...)
	b = appendF32(b, x)
	b = appendF32(b, y)
	b = appendF32(b, z)
	b = appendF32(b, pitch)
	b = appendF32(b, yaw)
	b = appendF32(b, headYaw)
	b = append(b, mode)
	if onGround {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	b = append(b, WriteZigzag64(uint64(riddenID))...)
	if mode == MoveModeTeleport {

		b = append(b, 0, 0, 0, 0)
		b = append(b, 0, 0, 0, 0)
	}
	b = append(b, WriteZigzag64(uint64(tick))...)
	return b
}

func BuildContainerClose(windowID byte, serverSide bool) []byte {
	b := WriteVarint(NTIDContainerClose)
	b = append(b, windowID)
	if serverSide {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	return b
}

func BuildInventoryTransactionUseItem(bx, by, bz int32, face int32, px, py, pz, cx, cy, cz float32) []byte {
	b := WriteVarint(NTIDInventoryTransaction)
	b = append(b, WriteZigzag32(-1)...)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteVarint(2)...)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteZigzag32(bx)...)
	b = append(b, WriteVarint(uint32(by))...)
	b = append(b, WriteZigzag32(bz)...)
	b = append(b, WriteZigzag32(face)...)
	b = append(b, WriteZigzag32(0)...)
	b = append(b, WriteZigzag32(0)...)
	b = appendF32(b, px)
	b = appendF32(b, py)
	b = appendF32(b, pz)
	b = appendF32(b, cx)
	b = appendF32(b, cy)
	b = appendF32(b, cz)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteVarint(0)...)
	return b
}

const (
	CommandBlockImpulse   = 0
	CommandBlockRepeating = 1
	CommandBlockChain     = 2
)

type CommandBlockOptions struct {
	Block              bool
	X, Y, Z            int32
	Mode               uint32
	NeedsRedstone      bool
	Conditional        bool
	MinecartRuntimeID  uint64
	Command            string
	LastOutput         string
	Name               string
	ShouldTrackOutput  bool
	TickDelay          uint32
	ExecuteOnFirstTick bool
}

func BuildCommandBlockUpdate(o CommandBlockOptions) []byte {
	b := WriteVarint(NTIDCommandBlockUpdate)
	if o.Block {
		b = append(b, 1)
	} else {
		b = append(b, 0)
	}
	if o.Block {
		b = append(b, WriteVarint(uint32(o.X))...)
		b = append(b, WriteVarint(uint32(o.Y))...)
		b = append(b, WriteVarint(uint32(o.Z))...)
		b = append(b, WriteVarint(o.Mode)...)
		b = appendBool(b, o.NeedsRedstone)
		b = appendBool(b, o.Conditional)
	} else {
		b = append(b, WriteVarint64(o.MinecartRuntimeID)...)
	}
	b = append(b, WriteString(o.Command)...)
	b = append(b, WriteString(o.LastOutput)...)
	b = append(b, WriteString(o.Name)...)
	b = append(b, WriteString("")...)
	b = appendBool(b, o.ShouldTrackOutput)
	b = append(b, WriteVarint(o.TickDelay)...)
	b = appendBool(b, o.ExecuteOnFirstTick)
	return b
}

func appendBool(b []byte, v bool) []byte {
	if v {
		return append(b, 1)
	}
	return append(b, 0)
}

func BuildItemStackRequest(requestID int32, actions [][]byte, customNames []string, cause int32) []byte {
	b := WriteVarint(NTIDItemStackRequest)
	b = append(b, WriteVarint(uint32(requestID))...)
	b = append(b, WriteVarint(uint32(len(actions)))...)
	for _, a := range actions {
		b = append(b, a...)
	}
	b = append(b, WriteVarint(uint32(len(customNames)))...)
	for _, n := range customNames {
		b = append(b, WriteString(n)...)
	}
	b = append(b, WriteZigzag32(cause)...)
	return b
}

const (
	AuthInputAscend                  = 0
	AuthInputDescend                 = 1
	AuthInputNorthJump               = 2
	AuthInputJumpDown                = 3
	AuthInputSprintDown              = 4
	AuthInputChangeHeight            = 5
	AuthInputJumping                 = 6
	AuthInputAutoJumpingInWater      = 7
	AuthInputSneaking                = 8
	AuthInputSneakDown               = 9
	AuthInputUp                      = 10
	AuthInputDown                    = 11
	AuthInputLeft                    = 12
	AuthInputRight                   = 13
	AuthInputUpLeft                  = 14
	AuthInputUpRight                 = 15
	AuthInputWantUp                  = 16
	AuthInputWantDown                = 17
	AuthInputWantDownSlow            = 18
	AuthInputWantUpSlow              = 19
	AuthInputSprinting               = 20
	AuthInputAscendScaffolding       = 21
	AuthInputDescendScaffolding      = 22
	AuthInputSneakToggleDown         = 23
	AuthInputPersistSneak            = 24
	AuthInputStartSprinting          = 25
	AuthInputStopSprinting           = 26
	AuthInputStartSneaking           = 27
	AuthInputStopSneaking            = 28
	AuthInputStartSwimming           = 29
	AuthInputStopSwimming            = 30
	AuthInputStartJumping            = 31
	AuthInputStartGliding            = 32
	AuthInputStopGliding             = 33
	AuthInputPerformItemInteraction  = 34
	AuthInputPerformBlockActions     = 35
	AuthInputPerformItemStackRequest = 36
)

func AuthKeyMode(flags ...int) uint64 {
	var m uint64
	for _, f := range flags {
		if f >= 0 && f < 64 {
			m |= 1 << uint(f)
		}
	}
	return m
}

func BuildPlayerAuthInputEx(
	pitch, yaw, x, y, z float32,
	moveVecX, moveVecY float32,
	headYaw float32,
	keyMode uint64,
	tick int64,
	deltaX, deltaY, deltaZ float32,
	cameraX, cameraY, cameraZ float32,
	onGround, resetPos bool) []byte {
	b := WriteVarint(NTIDPlayerAuthInput)
	b = appendF32(b, pitch)
	b = appendF32(b, yaw)
	b = appendF32(b, x)
	b = appendF32(b, y)
	b = appendF32(b, z)
	b = appendF32(b, moveVecX)
	b = appendF32(b, moveVecY)
	b = appendF32(b, headYaw)
	b = append(b, WriteVarint64(keyMode)...)
	b = append(b, WriteVarint(1)...)
	b = append(b, WriteVarint(0)...)
	b = append(b, WriteVarint(0)...)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = append(b, WriteZigzag64(uint64(tick))...)
	b = appendF32(b, deltaX)
	b = appendF32(b, deltaY)
	b = appendF32(b, deltaZ)
	b = appendBool(b, false)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendF32(b, cameraX)
	b = appendF32(b, cameraY)
	b = appendF32(b, cameraZ)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendBool(b, false)
	b = appendF32(b, 0)
	b = appendF32(b, 0)
	b = appendBool(b, false)
	b = appendBool(b, onGround)
	b = appendBool(b, resetPos)
	return b
}

package mc

const (
	NTIDLogin                             = 1
	NTIDPlayStatus                        = 2
	NTIDServerToClientHandshake           = 3
	NTIDClientToServerHandshake           = 4
	NTIDDisconnect                        = 5
	NTIDResourcePacksInfo                 = 6
	NTIDResourcePackStack                 = 7
	NTIDResourcePackClientResponse        = 8
	NTIDText                              = 9
	NTIDSetTime                           = 10
	NTIDStartGame                         = 11
	NTIDAddPlayer                         = 12
	NTIDAddActor                          = 13
	NTIDRemoveActor                       = 14
	NTIDAddItemActor                      = 15
	NTIDTakeItemActor                     = 17
	NTIDMoveActorAbsolute                 = 18
	NTIDMovePlayer                        = 19
	NTIDPassengerJump                     = 20
	NTIDUpdateBlock                       = 21
	NTIDAddPainting                       = 22
	NTIDTickSync                          = 23
	NTIDLevelSoundEventV1                 = 24
	NTIDLevelEvent                        = 25
	NTIDBlockEvent                        = 26
	NTIDActorEvent                        = 27
	NTIDMobEffect                         = 28
	NTIDUpdateAttributes                  = 29
	NTIDInventoryTransaction              = 30
	NTIDMobEquipment                      = 31
	NTIDMobArmourEquipment                = 32
	NTIDInteract                          = 33
	NTIDBlockPickRequest                  = 34
	NTIDActorPickRequest                  = 35
	NTIDPlayerAction                      = 36
	NTIDHurtArmour                        = 38
	NTIDSetActorData                      = 39
	NTIDSetActorMotion                    = 40
	NTIDSetActorLink                      = 41
	NTIDSetHealth                         = 42
	NTIDSetSpawnPosition                  = 43
	NTIDAnimate                           = 44
	NTIDRespawn                           = 45
	NTIDContainerOpen                     = 46
	NTIDContainerClose                    = 47
	NTIDPlayerHotBar                      = 48
	NTIDInventoryContent                  = 49
	NTIDInventorySlot                     = 50
	NTIDContainerSetData                  = 51
	NTIDCraftingData                      = 52
	NTIDCraftingEvent                     = 53
	NTIDGUIDataPickItem                   = 54
	NTIDAdventureSettings                 = 55
	NTIDBlockActorData                    = 56
	NTIDPlayerInput                       = 57
	NTIDLevelChunk                        = 58
	NTIDSetCommandsEnabled                = 59
	NTIDSetDifficulty                     = 60
	NTIDChangeDimension                   = 61
	NTIDSetPlayerGameType                 = 62
	NTIDPlayerList                        = 63
	NTIDSimpleEvent                       = 64
	NTIDEvent                             = 65
	NTIDSpawnExperienceOrb                = 66
	NTIDClientBoundMapItemData            = 67
	NTIDMapInfoRequest                    = 68
	NTIDRequestChunkRadius                = 69
	NTIDChunkRadiusUpdated                = 70
	NTIDItemFrameDropItem                 = 71
	NTIDGameRulesChanged                  = 72
	NTIDCamera                            = 73
	NTIDBossEvent                         = 74
	NTIDShowCredits                       = 75
	NTIDAvailableCommands                 = 76
	NTIDCommandRequest                    = 77
	NTIDCommandBlockUpdate                = 78
	NTIDCommandOutput                     = 79
	NTIDUpdateTrade                       = 80
	NTIDUpdateEquip                       = 81
	NTIDResourcePackDataInfo              = 82
	NTIDResourcePackChunkData             = 83
	NTIDResourcePackChunkRequest          = 84
	NTIDTransfer                          = 85
	NTIDPlaySound                         = 86
	NTIDStopSound                         = 87
	NTIDSetTitle                          = 88
	NTIDAddBehaviourTree                  = 89
	NTIDStructureBlockUpdate              = 90
	NTIDShowStoreOffer                    = 91
	NTIDPurchaseReceipt                   = 92
	NTIDPlayerSkin                        = 93
	NTIDSubClientLogin                    = 94
	NTIDAutomationClientConnect           = 95
	NTIDSetLastHurtBy                     = 96
	NTIDBookEdit                          = 97
	NTIDNPCRequest                        = 98
	NTIDPhotoTransfer                     = 99
	NTIDModalFormRequest                  = 100
	NTIDModalFormResponse                 = 101
	NTIDServerSettingsRequest             = 102
	NTIDServerSettingsResponse            = 103
	NTIDShowProfile                       = 104
	NTIDSetDefaultGameType                = 105
	NTIDRemoveObjective                   = 106
	NTIDSetDisplayObjective               = 107
	NTIDSetScore                          = 108
	NTIDLabTable                          = 109
	NTIDUpdateBlockSynced                 = 110
	NTIDMoveActorDelta                    = 111
	NTIDSetScoreboardIdentity             = 112
	NTIDSetLocalPlayerAsInitialised       = 113
	NTIDUpdateSoftEnum                    = 114
	NTIDNetworkStackLatency               = 115
	NTIDScriptCustomEvent                 = 117
	NTIDSpawnParticleEffect               = 118
	NTIDAvailableActorIdentifiers         = 119
	NTIDLevelSoundEventV2                 = 120
	NTIDNetworkChunkPublisherUpdate       = 121
	NTIDBiomeDefinitionList               = 122
	NTIDLevelSoundEvent                   = 123
	NTIDLevelEventGeneric                 = 124
	NTIDLecternUpdate                     = 125
	NTIDAddEntity                         = 127
	NTIDRemoveEntity                      = 128
	NTIDClientCacheStatus                 = 129
	NTIDOnScreenTextureAnimation          = 130
	NTIDMapCreateLockedCopy               = 131
	NTIDStructureTemplateDataRequest      = 132
	NTIDStructureTemplateDataResponse     = 133
	NTIDClientCacheBlobStatus             = 135
	NTIDClientCacheMissResponse           = 136
	NTIDEducationSettings                 = 137
	NTIDEmote                             = 138
	NTIDMultiPlayerSettings               = 139
	NTIDSettingsCommand                   = 140
	NTIDAnvilDamage                       = 141
	NTIDCompletedUsingItem                = 142
	NTIDNetworkSettings                   = 143
	NTIDPlayerAuthInput                   = 144
	NTIDCreativeContent                   = 145
	NTIDPlayerEnchantOptions              = 146
	NTIDItemStackRequest                  = 147
	NTIDItemStackResponse                 = 148
	NTIDPlayerArmourDamage                = 149
	NTIDCodeBuilder                       = 150
	NTIDUpdatePlayerGameType              = 151
	NTIDEmoteList                         = 152
	NTIDPositionTrackingDBServerBroadcast = 153
	NTIDPositionTrackingDBClientRequest   = 154
	NTIDDebugInfo                         = 155
	NTIDPacketViolationWarning            = 156
	NTIDMotionPredictionHints             = 157
	NTIDAnimateEntity                     = 158
	NTIDCameraShake                       = 159
	NTIDPlayerFog                         = 160
	NTIDCorrectPlayerMovePrediction       = 161
	NTIDItemComponent                     = 162
	NTIDFilterText                        = 163
	NTIDClientBoundDebugRenderer          = 164
	NTIDSyncActorProperty                 = 165
	NTIDAddVolumeEntity                   = 166
	NTIDRemoveVolumeEntity                = 167
	NTIDSimulationType                    = 168
	NTIDNPCDialogue                       = 169
	NTIDEducationResourceURI              = 170
	NTIDCreatePhoto                       = 171
	NTIDUpdateSubChunkBlocks              = 172
	NTIDPhotoInfoRequest                  = 173
	NTIDSubChunk                          = 174
	NTIDSubChunkRequest                   = 175
	NTIDClientStartItemCooldown           = 176
	NTIDScriptMessage                     = 177
	NTIDCodeBuilderSource                 = 178
	NTIDTickingAreasLoadStatus            = 179
	NTIDDimensionData                     = 180
	NTIDAgentAction                       = 181
	NTIDChangeMobProperty                 = 182
	NTIDLessonProgress                    = 183
	NTIDRequestAbility                    = 184
	NTIDRequestPermissions                = 185
	NTIDToastRequest                      = 186
	NTIDUpdateAbilities                   = 187
	NTIDUpdateAdventureSettings           = 188
	NTIDDeathInfo                         = 189
	NTIDEditorNetwork                     = 190
	NTIDFeatureRegistry                   = 191
	NTIDServerStats                       = 192
	NTIDRequestNetworkSettings            = 193
	NTIDGameTestRequest                   = 194
	NTIDGameTestResults                   = 195
	NTIDUpdateClientInputLocks            = 196
	NTIDClientCheatAbility                = 197
	NTIDCameraPresets                     = 198
	NTIDUnlockedRecipes                   = 199
	NTIDPyRpc                             = 200
	NTIDChangeModel                       = 201
	NTIDStoreBuySucc                      = 202
	NTIDNeteaseJson                       = 203
	NTIDChangeModelTexture                = 204
	NTIDChangeModelOffset                 = 205
	NTIDChangeModelBind                   = 206
	NTIDHungerAttr                        = 207
	NTIDSetDimensionLocalTime             = 208
	NTIDWithdrawFurnaceXp                 = 209
	NTIDSetDimensionLocalWeather          = 210
	NTIDCustomV1                          = 223
	NTIDCombine                           = 224
	NTIDVConnection                       = 225
	NTIDTransport                         = 226
	NTIDCustomV2                          = 227
	NTIDConfirmSkin                       = 228
	NTIDTransportNoCompress               = 229
	NTIDMobEffectV2                       = 230
	NTIDMobBlockActorChanged              = 231
	NTIDChangeActorMotion                 = 232
	NTIDAnimateEmoteEntity                = 233
	NTIDCameraInstruction                 = 300
	NTIDCompressedBiomeDefinitionList     = 301
	NTIDTrimData                          = 302
	NTIDOpenSign                          = 303
	NTIDAgentAnimation                    = 304
)

var NTIDByNumber = map[int]string{
	1:   "Login",
	2:   "PlayStatus",
	3:   "ServerToClientHandshake",
	4:   "ClientToServerHandshake",
	5:   "Disconnect",
	6:   "ResourcePacksInfo",
	7:   "ResourcePackStack",
	8:   "ResourcePackClientResponse",
	9:   "Text",
	10:  "SetTime",
	11:  "StartGame",
	12:  "AddPlayer",
	13:  "AddActor",
	14:  "RemoveActor",
	15:  "AddItemActor",
	17:  "TakeItemActor",
	18:  "MoveActorAbsolute",
	19:  "MovePlayer",
	20:  "PassengerJump",
	21:  "UpdateBlock",
	22:  "AddPainting",
	23:  "TickSync",
	24:  "LevelSoundEventV1",
	25:  "LevelEvent",
	26:  "BlockEvent",
	27:  "ActorEvent",
	28:  "MobEffect",
	29:  "UpdateAttributes",
	30:  "InventoryTransaction",
	31:  "MobEquipment",
	32:  "MobArmourEquipment",
	33:  "Interact",
	34:  "BlockPickRequest",
	35:  "ActorPickRequest",
	36:  "PlayerAction",
	38:  "HurtArmour",
	39:  "SetActorData",
	40:  "SetActorMotion",
	41:  "SetActorLink",
	42:  "SetHealth",
	43:  "SetSpawnPosition",
	44:  "Animate",
	45:  "Respawn",
	46:  "ContainerOpen",
	47:  "ContainerClose",
	48:  "PlayerHotBar",
	49:  "InventoryContent",
	50:  "InventorySlot",
	51:  "ContainerSetData",
	52:  "CraftingData",
	53:  "CraftingEvent",
	54:  "GUIDataPickItem",
	55:  "AdventureSettings",
	56:  "BlockActorData",
	57:  "PlayerInput",
	58:  "LevelChunk",
	59:  "SetCommandsEnabled",
	60:  "SetDifficulty",
	61:  "ChangeDimension",
	62:  "SetPlayerGameType",
	63:  "PlayerList",
	64:  "SimpleEvent",
	65:  "Event",
	66:  "SpawnExperienceOrb",
	67:  "ClientBoundMapItemData",
	68:  "MapInfoRequest",
	69:  "RequestChunkRadius",
	70:  "ChunkRadiusUpdated",
	71:  "ItemFrameDropItem",
	72:  "GameRulesChanged",
	73:  "Camera",
	74:  "BossEvent",
	75:  "ShowCredits",
	76:  "AvailableCommands",
	77:  "CommandRequest",
	78:  "CommandBlockUpdate",
	79:  "CommandOutput",
	80:  "UpdateTrade",
	81:  "UpdateEquip",
	82:  "ResourcePackDataInfo",
	83:  "ResourcePackChunkData",
	84:  "ResourcePackChunkRequest",
	85:  "Transfer",
	86:  "PlaySound",
	87:  "StopSound",
	88:  "SetTitle",
	89:  "AddBehaviourTree",
	90:  "StructureBlockUpdate",
	91:  "ShowStoreOffer",
	92:  "PurchaseReceipt",
	93:  "PlayerSkin",
	94:  "SubClientLogin",
	95:  "AutomationClientConnect",
	96:  "SetLastHurtBy",
	97:  "BookEdit",
	98:  "NPCRequest",
	99:  "PhotoTransfer",
	100: "ModalFormRequest",
	101: "ModalFormResponse",
	102: "ServerSettingsRequest",
	103: "ServerSettingsResponse",
	104: "ShowProfile",
	105: "SetDefaultGameType",
	106: "RemoveObjective",
	107: "SetDisplayObjective",
	108: "SetScore",
	109: "LabTable",
	110: "UpdateBlockSynced",
	111: "MoveActorDelta",
	112: "SetScoreboardIdentity",
	113: "SetLocalPlayerAsInitialised",
	114: "UpdateSoftEnum",
	115: "NetworkStackLatency",
	117: "ScriptCustomEvent",
	118: "SpawnParticleEffect",
	119: "AvailableActorIdentifiers",
	120: "LevelSoundEventV2",
	121: "NetworkChunkPublisherUpdate",
	122: "BiomeDefinitionList",
	123: "LevelSoundEvent",
	124: "LevelEventGeneric",
	125: "LecternUpdate",
	127: "AddEntity",
	128: "RemoveEntity",
	129: "ClientCacheStatus",
	130: "OnScreenTextureAnimation",
	131: "MapCreateLockedCopy",
	132: "StructureTemplateDataRequest",
	133: "StructureTemplateDataResponse",
	135: "ClientCacheBlobStatus",
	136: "ClientCacheMissResponse",
	137: "EducationSettings",
	138: "Emote",
	139: "MultiPlayerSettings",
	140: "SettingsCommand",
	141: "AnvilDamage",
	142: "CompletedUsingItem",
	143: "NetworkSettings",
	144: "PlayerAuthInput",
	145: "CreativeContent",
	146: "PlayerEnchantOptions",
	147: "ItemStackRequest",
	148: "ItemStackResponse",
	149: "PlayerArmourDamage",
	150: "CodeBuilder",
	151: "UpdatePlayerGameType",
	152: "EmoteList",
	153: "PositionTrackingDBServerBroadcast",
	154: "PositionTrackingDBClientRequest",
	155: "DebugInfo",
	156: "PacketViolationWarning",
	157: "MotionPredictionHints",
	158: "AnimateEntity",
	159: "CameraShake",
	160: "PlayerFog",
	161: "CorrectPlayerMovePrediction",
	162: "ItemComponent",
	163: "FilterText",
	164: "ClientBoundDebugRenderer",
	165: "SyncActorProperty",
	166: "AddVolumeEntity",
	167: "RemoveVolumeEntity",
	168: "SimulationType",
	169: "NPCDialogue",
	170: "EducationResourceURI",
	171: "CreatePhoto",
	172: "UpdateSubChunkBlocks",
	173: "PhotoInfoRequest",
	174: "SubChunk",
	175: "SubChunkRequest",
	176: "ClientStartItemCooldown",
	177: "ScriptMessage",
	178: "CodeBuilderSource",
	179: "TickingAreasLoadStatus",
	180: "DimensionData",
	181: "AgentAction",
	182: "ChangeMobProperty",
	183: "LessonProgress",
	184: "RequestAbility",
	185: "RequestPermissions",
	186: "ToastRequest",
	187: "UpdateAbilities",
	188: "UpdateAdventureSettings",
	189: "DeathInfo",
	190: "EditorNetwork",
	191: "FeatureRegistry",
	192: "ServerStats",
	193: "RequestNetworkSettings",
	194: "GameTestRequest",
	195: "GameTestResults",
	196: "UpdateClientInputLocks",
	197: "ClientCheatAbility",
	198: "CameraPresets",
	199: "UnlockedRecipes",
	200: "PyRpc",
	201: "ChangeModel",
	202: "StoreBuySucc",
	203: "NeteaseJson",
	204: "ChangeModelTexture",
	205: "ChangeModelOffset",
	206: "ChangeModelBind",
	207: "HungerAttr",
	208: "SetDimensionLocalTime",
	209: "WithdrawFurnaceXp",
	210: "SetDimensionLocalWeather",
	223: "CustomV1",
	224: "Combine",
	225: "VConnection",
	226: "Transport",
	227: "CustomV2",
	228: "ConfirmSkin",
	229: "TransportNoCompress",
	230: "MobEffectV2",
	231: "MobBlockActorChanged",
	232: "ChangeActorMotion",
	233: "AnimateEmoteEntity",
	300: "CameraInstruction",
	301: "CompressedBiomeDefinitionList",
	302: "TrimData",
	303: "OpenSign",
	304: "AgentAnimation",
}

func NTIDName(id int) string { return NTIDByNumber[id] }

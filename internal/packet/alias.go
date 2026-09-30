package packet

// The canonical AO2 packet types and enums come from github.com/SyntaxNyah/aolib-go
// (generated from aolib-meta). This file re-exports them under the local
// `packet` name so the rest of Athena keeps importing one package, while the
// wire types stay in lockstep with aolib-go / aolib-ts.
//
// MS (MSToServer/MSToClient) is NOT aliased here: it stays hand-written in
// mspacket.go because it carries Nyathena extensions (blips, additional_chars,
// custom shout name, "pid^order" paired_charid) that canonical aolib does not
// model. The Athena-only TT/SETCASE/CASEA packets live in athena.go for the
// same reason. Everything else is the canonical wire form.

import aolib "github.com/SyntaxNyah/aolib-go"

// Shared object type.
type Offset = aolib.Offset

// Enums.
type (
	AreaUpdateType = aolib.AreaUpdateType
	DeskModifier   = aolib.DeskModifier
	EmoteModifier  = aolib.EmoteModifier
	Flip           = aolib.Flip
	ShoutModifier  = aolib.ShoutModifier
	Side           = aolib.Side
	TextColor      = aolib.TextColor
)

const (
	AreaUpdateTypePlayerCount = aolib.AreaUpdateTypePlayerCount
	AreaUpdateTypeStatus      = aolib.AreaUpdateTypeStatus
	AreaUpdateTypeCaseManager = aolib.AreaUpdateTypeCaseManager
	AreaUpdateTypeLocked      = aolib.AreaUpdateTypeLocked

	DeskModifierHidden                      = aolib.DeskModifierHidden
	DeskModifierShown                       = aolib.DeskModifierShown
	DeskModifierHideDuringPreanim           = aolib.DeskModifierHideDuringPreanim
	DeskModifierShowDuringPreanim           = aolib.DeskModifierShowDuringPreanim
	DeskModifierHideAndCenterDuringPreanim  = aolib.DeskModifierHideAndCenterDuringPreanim
	DeskModifierShowDuringPreanimThenCenter = aolib.DeskModifierShowDuringPreanimThenCenter

	EmoteModifierNoPreanim           = aolib.EmoteModifierNoPreanim
	EmoteModifierPreanim             = aolib.EmoteModifierPreanim
	EmoteModifierPreanimAndObjection = aolib.EmoteModifierPreanimAndObjection
	EmoteModifierUnused3             = aolib.EmoteModifierUnused3
	EmoteModifierUnused4             = aolib.EmoteModifierUnused4
	EmoteModifierZoom                = aolib.EmoteModifierZoom
	EmoteModifierObjectionZoom       = aolib.EmoteModifierObjectionZoom

	FlipNone                  = aolib.FlipNone
	FlipHorizontal            = aolib.FlipHorizontal
	FlipVertical              = aolib.FlipVertical
	FlipHorizontalAndVertical = aolib.FlipHorizontalAndVertical

	ShoutModifierNone      = aolib.ShoutModifierNone
	ShoutModifierHoldIt    = aolib.ShoutModifierHoldIt
	ShoutModifierObjection = aolib.ShoutModifierObjection
	ShoutModifierTakeThat  = aolib.ShoutModifierTakeThat
	ShoutModifierCustom    = aolib.ShoutModifierCustom

	SideDef = aolib.SideDef
	SidePro = aolib.SidePro
	SideHld = aolib.SideHld
	SideHlp = aolib.SideHlp
	SideWit = aolib.SideWit
	SideJud = aolib.SideJud
	SideJur = aolib.SideJur
	SideSea = aolib.SideSea

	TextColorWhite   = aolib.TextColorWhite
	TextColorGreen   = aolib.TextColorGreen
	TextColorRed     = aolib.TextColorRed
	TextColorOrange  = aolib.TextColorOrange
	TextColorBlue    = aolib.TextColorBlue
	TextColorYellow  = aolib.TextColorYellow
	TextColorPink    = aolib.TextColorPink
	TextColorCyan    = aolib.TextColorCyan
	TextColorGrey    = aolib.TextColorGrey
	TextColorRainbow = aolib.TextColorRainbow
)

// Canonical packets (MS excluded; see mspacket.go).
type (
	ARUP               = aolib.ARUP
	ASS                = aolib.ASS
	AUTH               = aolib.AUTH
	Askchaa            = aolib.Askchaa
	BB                 = aolib.BB
	BD                 = aolib.BD
	BN                 = aolib.BN
	CC                 = aolib.CC
	CH                 = aolib.CH
	CHECK              = aolib.CHECK
	CI                 = aolib.CI
	CTToClient         = aolib.CTToClient
	CTToServer         = aolib.CTToServer
	CharsCheck         = aolib.CharsCheck
	DE                 = aolib.DE
	DONE               = aolib.DONE
	Decryptor          = aolib.Decryptor
	EE                 = aolib.EE
	EI                 = aolib.EI
	EM                 = aolib.EM
	FA                 = aolib.FA
	FL                 = aolib.FL
	FM                 = aolib.FM
	HI                 = aolib.HI
	HPToClient         = aolib.HPToClient
	HPToServer         = aolib.HPToServer
	IDToClient         = aolib.IDToClient
	IDToServer         = aolib.IDToServer
	JD                 = aolib.JD
	KB                 = aolib.KB
	KK                 = aolib.KK
	LE                 = aolib.LE
	MA                 = aolib.MA
	MCToClient         = aolib.MCToClient
	MCToServer         = aolib.MCToServer
	PE                 = aolib.PE
	PN                 = aolib.PN
	PR                 = aolib.PR
	PU                 = aolib.PU
	PV                 = aolib.PV
	RC                 = aolib.RC
	RD                 = aolib.RD
	RM                 = aolib.RM
	RMC                = aolib.RMC
	RTToClient         = aolib.RTToClient
	RTToServer         = aolib.RTToServer
	SC                 = aolib.SC
	SI                 = aolib.SI
	SM                 = aolib.SM
	SP                 = aolib.SP
	TI                 = aolib.TI
	VS_AUDIO           = aolib.VS_AUDIO
	VS_CAPS            = aolib.VS_CAPS
	VS_FRAME           = aolib.VS_FRAME
	VS_JOINToClient    = aolib.VS_JOINToClient
	VS_JOINToServer    = aolib.VS_JOINToServer
	VS_LEAVEToClient   = aolib.VS_LEAVEToClient
	VS_LEAVEToServer   = aolib.VS_LEAVEToServer
	VS_PEERS           = aolib.VS_PEERS
	VS_SPEAKToClient   = aolib.VS_SPEAKToClient
	VS_SPEAKToServer   = aolib.VS_SPEAKToServer
	ZZToClient         = aolib.ZZToClient
	ZZToServer         = aolib.ZZToServer
)

// Positional (FantaCode) decoders for the canonical packets. MS decoders stay
// in mspacket.go.
var (
	ParseARUP             = aolib.ParseARUP
	ParseASS              = aolib.ParseASS
	ParseAUTH             = aolib.ParseAUTH
	ParseAskchaa          = aolib.ParseAskchaa
	ParseBB               = aolib.ParseBB
	ParseBD               = aolib.ParseBD
	ParseBN               = aolib.ParseBN
	ParseCC               = aolib.ParseCC
	ParseCH               = aolib.ParseCH
	ParseCHECK            = aolib.ParseCHECK
	ParseCI               = aolib.ParseCI
	ParseCTToClient       = aolib.ParseCTToClient
	ParseCTToServer       = aolib.ParseCTToServer
	ParseCharsCheck       = aolib.ParseCharsCheck
	ParseDE               = aolib.ParseDE
	ParseDONE             = aolib.ParseDONE
	ParseDecryptor        = aolib.ParseDecryptor
	ParseEE               = aolib.ParseEE
	ParseEI               = aolib.ParseEI
	ParseEM               = aolib.ParseEM
	ParseFA               = aolib.ParseFA
	ParseFL               = aolib.ParseFL
	ParseFM               = aolib.ParseFM
	ParseHI               = aolib.ParseHI
	ParseHPToClient       = aolib.ParseHPToClient
	ParseHPToServer       = aolib.ParseHPToServer
	ParseIDToClient       = aolib.ParseIDToClient
	ParseIDToServer       = aolib.ParseIDToServer
	ParseJD               = aolib.ParseJD
	ParseKB               = aolib.ParseKB
	ParseKK               = aolib.ParseKK
	ParseLE               = aolib.ParseLE
	ParseMA               = aolib.ParseMA
	ParseMCToClient       = aolib.ParseMCToClient
	ParseMCToServer       = aolib.ParseMCToServer
	ParsePE               = aolib.ParsePE
	ParsePN               = aolib.ParsePN
	ParsePR               = aolib.ParsePR
	ParsePU               = aolib.ParsePU
	ParsePV               = aolib.ParsePV
	ParseRC               = aolib.ParseRC
	ParseRD               = aolib.ParseRD
	ParseRM               = aolib.ParseRM
	ParseRMC              = aolib.ParseRMC
	ParseRTToClient       = aolib.ParseRTToClient
	ParseRTToServer       = aolib.ParseRTToServer
	ParseSC               = aolib.ParseSC
	ParseSI               = aolib.ParseSI
	ParseSM               = aolib.ParseSM
	ParseSP               = aolib.ParseSP
	ParseTI               = aolib.ParseTI
	ParseVS_AUDIO         = aolib.ParseVS_AUDIO
	ParseVS_CAPS          = aolib.ParseVS_CAPS
	ParseVS_FRAME         = aolib.ParseVS_FRAME
	ParseVS_JOINToClient  = aolib.ParseVS_JOINToClient
	ParseVS_JOINToServer  = aolib.ParseVS_JOINToServer
	ParseVS_LEAVEToClient = aolib.ParseVS_LEAVEToClient
	ParseVS_LEAVEToServer = aolib.ParseVS_LEAVEToServer
	ParseVS_PEERS         = aolib.ParseVS_PEERS
	ParseVS_SPEAKToClient = aolib.ParseVS_SPEAKToClient
	ParseVS_SPEAKToServer = aolib.ParseVS_SPEAKToServer
	ParseZZToClient       = aolib.ParseZZToClient
	ParseZZToServer       = aolib.ParseZZToServer
)

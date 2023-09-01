package model

import "horizon/structure"

type (
	ShardNumResponse struct {
		ShardNum uint
	}

	ConsensusFlagResponse struct {
		Flag bool
	}

	MessageMetaData struct {
		MessageType uint
		Message     []byte
	}

	// MessageReady MessageType = 0
	MessageReady struct {
		PersonalID string
		IdList     []string
	}

	//MessageType = 1
	MessageIsWin struct {
		IsWin       bool
		IsConsensus bool
		WinID       string
		PersonalID  string
		IdList      []string
		Shardnum    int
	}

	//MessageType = 2
	MultiCastBlockRequest struct {
		Shard uint
		Id    string
		Block structure.Block
	}
	MultiCastBlockResponse struct {
		Message string
	}

	//MessageType = 3
	SendVoteRequest struct {
		Shard       uint
		BlockHeight int
		WinID       string
		PersonalID  string
		Agree       bool
	}

	SendVoteResponse struct {
		Message string
	}

	HeightResponse struct {
		Height int
	}
)

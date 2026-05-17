//
// File:        internal/kosync/websocket.go
// Project:     https://git.obth.eu/atjontv/kosync
// Copyright:   © 2025-2026 Thomas Obernosterer. Licensed under the EUPL-1.2 or later
//

package kosync

type PubSubTopic int

const (
	PubSubTopicUnknown PubSubTopic = iota
	PubSubTopicUserDocuments
	PubSubTopicUserStatistics
	PubSubTopicsCount
)

var PubSubTopicStrings = map[PubSubTopic]string{
	PubSubTopicUnknown:       "",
	PubSubTopicUserDocuments: "user.documents",
	PubSubTopicUserStatistics: "user.statistics",
}

// comptime assert PubSubTopicStrings has same length as PubSubTopicsCount
var _ = [1]int{}[len(PubSubTopicStrings)-int(PubSubTopicsCount)]

type WsInfo struct {
	ServerName    string `json:"server_name"`
	ServerVersion string `json:"server_version"`
	Message       string `json:"message"`
}

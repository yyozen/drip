package protocol

import (
	json "github.com/goccy/go-json"
)

type FlowControlAction string

const (
	FlowControlPause  FlowControlAction = "pause"
	FlowControlResume FlowControlAction = "resume"
)

type FlowControlMessage struct {
	StreamID string            `json:"stream_id"`
	Action   FlowControlAction `json:"action"`
}

func NewFlowControlFrame(streamID string, action FlowControlAction) *Frame {
	msg := FlowControlMessage{
		StreamID: streamID,
		Action:   action,
	}
	payload, _ := json.Marshal(&msg)
	return NewFrame(FrameTypeFlowControl, payload)
}

func DecodeFlowControlMessage(payload []byte) (*FlowControlMessage, error) {
	var msg FlowControlMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

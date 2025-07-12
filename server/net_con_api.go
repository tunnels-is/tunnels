package main

type netConMessage struct {
	Version int         `json:"version"`
	Method  string      `json:"method"`
	Data    interface{} `json:"data"`
}

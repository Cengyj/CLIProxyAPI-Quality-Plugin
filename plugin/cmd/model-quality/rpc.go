package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/router-for-me/CLIProxyAPI/v8/sdk/pluginabi"
)

func decodeHostResponse(raw []byte, status int, response any) error {
	var env pluginabi.Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("invalid host envelope")
	}
	if !env.OK || status != 0 || env.Error != nil {
		if env.Error != nil {
			return fmt.Errorf("%s: %w", env.Error.Code, env.Error)
		}
		return fmt.Errorf("host operation failed")
	}
	if response == nil {
		return nil
	}
	if len(env.Result) == 0 || bytes.Equal(bytes.TrimSpace(env.Result), []byte("null")) {
		return fmt.Errorf("host returned no result")
	}
	if err := json.Unmarshal(env.Result, response); err != nil {
		return fmt.Errorf("invalid host result")
	}
	return nil
}

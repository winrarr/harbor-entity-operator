/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"os/exec"
	"testing"
)

func TestCommandForLoggingRedactsBasicAuth(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "short flag",
			args: []string{"curl", "-u", "admin:secret", "https://harbor.example/api"},
			want: "curl -u [REDACTED] https://harbor.example/api",
		},
		{
			name: "long flag",
			args: []string{"curl", "--user", "admin:secret", "https://harbor.example/api"},
			want: "curl --user [REDACTED] https://harbor.example/api",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &exec.Cmd{Args: tt.args}
			if got := commandForLogging(cmd); got != tt.want {
				t.Fatalf("commandForLogging() = %q, want %q", got, tt.want)
			}
			if cmd.Args[2] != "admin:secret" {
				t.Fatal("commandForLogging mutated the command arguments")
			}
		})
	}
}

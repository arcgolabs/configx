package hcl

import (
	"os"
	"path/filepath"
	"testing"

	configx "github.com/arcgolabs/configx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithHCLSupport(t *testing.T) {
	cases := []struct {
		name     string
		ext      string
		content  string
		wantName string
		wantPort int
	}{
		{
			name:     "hcl",
			ext:      ".hcl",
			content:  "name = \"hcl\"\nport = 8080\n",
			wantName: "hcl",
			wantPort: 8080,
		},
		{
			name:     "tf",
			ext:      ".tf",
			content:  "name = \"tf\"\nport = 8081\n",
			wantName: "tf",
			wantPort: 8081,
		},
		{
			name:     "tfvars",
			ext:      ".tfvars",
			content:  "name = \"tfvars\"\nport = 9090\n",
			wantName: "tfvars",
			wantPort: 9090,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config"+tc.ext)
			require.NoError(t, os.WriteFile(path, []byte(tc.content), 0o600))

			cfg, err := configx.LoadConfig(
				WithHCLSupport(),
				configx.WithFiles(path),
			)
			require.NoError(t, err)
			assert.Equal(t, tc.wantName, cfg.GetString("name"))
			assert.Equal(t, tc.wantPort, cfg.GetInt("port"))
		})
	}
}

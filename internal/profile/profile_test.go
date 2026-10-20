package profile

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/codedb-io/codedb-lens/internal/adapter"
)

func TestPasswordNeverSerialized(t *testing.T) {
	f := File{}
	f.Put(Profile{Name: "prod", Conn: adapter.Conn{Engine: "mssql", Server: "s", User: "u", Password: "secret", DSN: "postgres://u:secret@h"}})
	raw, _ := json.Marshal(f)
	if strings.Contains(string(raw), "secret") {
		t.Fatalf("password leaked into profiles.json: %s", raw)
	}
	f.Put(Profile{Name: "dev"})
	if f.Last != "dev" || len(f.Profiles) != 2 || f.Profiles[0].Name != "dev" {
		t.Fatalf("Put: %+v", f)
	}
	f.Delete("dev")
	if _, ok := f.Get("dev"); ok || f.Last != "" {
		t.Fatalf("Delete: %+v", f)
	}
}

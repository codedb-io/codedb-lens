package adapter

import "testing"

func TestTarget(t *testing.T) {
	for _, tc := range []struct {
		c    Conn
		want string
	}{
		{Conn{Server: "sql01,1433"}, "sql01,1433"},
		{Conn{Server: "localhost", DSN: "postgres://lens:s3cret@db.example.com:5432/app"}, "db.example.com"},
		{Conn{DSN: "host=pg.internal user=lens password=s3cret"}, "pg.internal"},
		{Conn{DSN: "postgres://%zz"}, "DSN"}, // unparsable: never echo it, it may hold a password
	} {
		if got := tc.c.Target(); got != tc.want {
			t.Errorf("Target(%+v) = %q, want %q", tc.c, got, tc.want)
		}
	}
}

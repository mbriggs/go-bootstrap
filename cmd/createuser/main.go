// Command createuser provisions a login from the command line — the way the
// first admin gets created on a fresh database.
//
// Usage:
//
//	go run ./cmd/createuser -email you@example.com -password secret -roles admin
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/mbriggs/go-bootstrap/auth"
	"github.com/mbriggs/go-bootstrap/db"
)

func main() {
	os.Exit(run())
}

func run() int {
	email := flag.String("email", "", "email address (required)")
	password := flag.String("password", "", "password (required)")
	name := flag.String("name", "", "display name (defaults to the email local part)")
	roles := flag.String("roles", "", "comma-separated roles, e.g. admin")
	flag.Parse()

	ctx := context.Background()

	if err := db.Configure(ctx, ""); err != nil {
		fmt.Fprintln(os.Stderr, "createuser: db:", err)
		return 1
	}
	defer db.Close()

	var roleList []string
	for role := range strings.SplitSeq(*roles, ",") {
		if role = strings.TrimSpace(role); role != "" {
			roleList = append(roleList, role)
		}
	}

	user, err := auth.Create(ctx, auth.CreateInput{
		Email:    *email,
		Password: *password,
		Name:     *name,
		Roles:    roleList,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "createuser:", err)
		return 1
	}

	fmt.Printf("created user %d (%s)\n", user.ID, user.Email)

	return 0
}

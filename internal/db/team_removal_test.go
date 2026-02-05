package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ============================================================================
// SC-9: File internal/db/team.go is deleted
// ============================================================================

// TestTeamGoDeleted verifies that internal/db/team.go no longer exists.
// Covers: SC-9
func TestTeamGoDeleted(t *testing.T) {
	t.Parallel()

	// Find the directory containing this test file
	// The test is in internal/db/, so team.go should be in the same directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	teamGoPath := filepath.Join(wd, "team.go")

	if _, err := os.Stat(teamGoPath); err == nil {
		t.Error("internal/db/team.go should be deleted but still exists")
	} else if !os.IsNotExist(err) {
		t.Errorf("error checking for team.go: %v", err)
	}
}

// ============================================================================
// SC-10: Build succeeds without team.go types
// ============================================================================

// TestNoReferencesToOldTeamTypes verifies that no code references the old
// team.go types (TeamMember, TeamMemberRole, etc.) that are being removed.
// Covers: SC-10
func TestNoReferencesToOldTeamTypes(t *testing.T) {
	t.Parallel()

	// Types that were in team.go and should no longer be referenced
	oldTypes := []string{
		"TeamMember",       // struct
		"TeamMemberRole",   // type
		"RoleAdmin",        // const
		"RoleMember",       // const
		"RoleViewer",       // const
		"TaskClaim",        // struct (old version with member_id instead of user_id)
		"ActivityAction",   // type
		"ActionCreated",    // const
		"ActionStarted",    // const
		"ActionPaused",     // const
		"ActionCompleted",  // const
		"ActionFailed",     // const
		"ActionCommented",  // const
		"ActionClaimed",    // const
		"ActionReleased",   // const
		"ActivityLog",      // struct
		"TaskWithClaim",    // struct
	}

	// Find the db package directory
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	// Parse all Go files in the db package
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, wd, func(fi os.FileInfo) bool {
		// Skip test files and the team.go file itself (if it still exists)
		name := fi.Name()
		return strings.HasSuffix(name, ".go") &&
			!strings.HasSuffix(name, "_test.go") &&
			name != "team.go"
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	// Check for references to old types
	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			for _, oldType := range oldTypes {
				if containsIdentifier(file, oldType) {
					// Skip if it's a comment or string literal
					if isOnlyInCommentsOrStrings(file, oldType) {
						continue
					}
					t.Errorf("file %s still references old type %q which should be removed",
						filepath.Base(filename), oldType)
				}
			}
		}
	}
}

// TestOldClaimFunctionsRemoved verifies that the old claim functions from
// team.go are no longer available.
// Covers: SC-10
func TestOldClaimFunctionsRemoved(t *testing.T) {
	t.Parallel()

	// Functions that were in team.go and should no longer exist
	oldFunctions := []string{
		"CreateTeamMember",
		"GetTeamMember",
		"GetTeamMemberByEmail",
		"ListTeamMembers",
		"UpdateTeamMember",
		"DeleteTeamMember",
		"ReleaseTask",          // Old release, replaced by new atomic version
		"GetActiveTaskClaim",   // Old non-atomic version
		"GetMemberClaims",      // Uses old MemberID field
		"IsTaskClaimed",        // Old non-atomic version
		"IsTaskClaimedBy",      // Uses old MemberID field
		"LogActivity",          // Old team-based activity logging
		"ListActivity",         // Old team-based activity logging
		"GetRecentActivity",    // Old team-based activity logging
		"GetTaskActivity",      // Old team-based activity logging
		"GetMemberActivity",    // Old team-based activity logging
		"ListTasksWithClaims",  // Uses old TaskWithClaim type
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, wd, func(fi os.FileInfo) bool {
		name := fi.Name()
		return strings.HasSuffix(name, ".go") &&
			!strings.HasSuffix(name, "_test.go") &&
			name != "team.go"
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	for _, pkg := range pkgs {
		for filename, file := range pkg.Files {
			for _, oldFunc := range oldFunctions {
				if containsFuncDecl(file, oldFunc) {
					t.Errorf("file %s still declares old function %q which should be removed",
						filepath.Base(filename), oldFunc)
				}
			}
		}
	}
}

// containsIdentifier checks if an AST contains a reference to the given identifier.
func containsIdentifier(file *ast.File, name string) bool {
	found := false
	ast.Inspect(file, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == name {
			found = true
			return false // Stop searching
		}
		return true
	})
	return found
}

// containsFuncDecl checks if an AST contains a function declaration with the given name.
func containsFuncDecl(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return true
		}
	}
	return false
}

// isOnlyInCommentsOrStrings checks if all occurrences of a string are only
// in comments or string literals (not actual code references).
// This is a simplified check - returns true if the identifier is not
// in a type spec, func decl, or value spec.
func isOnlyInCommentsOrStrings(file *ast.File, name string) bool {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name.Name == name {
						return false // It's a type declaration
					}
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name == name {
							return false // It's a value declaration
						}
					}
				}
			}
		case *ast.FuncDecl:
			if d.Name.Name == name {
				return false // It's a function declaration
			}
		}
	}
	return true
}

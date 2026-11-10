package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"baramail/backend/internal/config"
	"baramail/backend/internal/database"
	"baramail/backend/internal/jobs"
	"baramail/backend/internal/repositories"
	"baramail/backend/internal/services"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	db, err := database.Connect(cfg.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx := context.Background()

	switch os.Args[1] {
	case "migrate":
		if err := database.RunMigrations(ctx, db, "migrations"); err != nil {
			fmt.Fprintf(os.Stderr, "Migration failed: %v\n", err)
			os.Exit(1)
		}
		if err := database.SeedDomains(ctx, db, cfg.AllowedEmailDomains); err != nil {
			fmt.Fprintf(os.Stderr, "Domain seed failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Migrations and initial domain seeding completed successfully.")

	case "cleanup":
		adminRepo := repositories.NewAdminRepository(db)
		msgRepo := repositories.NewMessageRepository(db)
		runner := jobs.NewCleanupRunner(cfg, msgRepo, adminRepo, nil)
		runner.RunOnce(ctx)
		fmt.Println("Cleanup job completed successfully.")

	case "admin":
		if len(os.Args) < 3 {
			printUsage()
			os.Exit(1)
		}
		adminRepo := repositories.NewAdminRepository(db)
		sub := os.Args[2]
		switch sub {
		case "create":
			fs := flag.NewFlagSet("admin create", flag.ExitOnError)
			username := fs.String("username", "", "Admin username")
			password := fs.String("password", "", "Admin password (min 8 chars)")
			email := fs.String("email", "", "Optional admin email")
			_ = fs.Parse(os.Args[3:])

			u := strings.TrimSpace(*username)
			p := *password
			if u == "" || len(p) < 8 {
				fmt.Fprintln(os.Stderr, "Error: --username is required and --password must be at least 8 characters.")
				os.Exit(1)
			}
			hash, err := services.HashPassword(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
				os.Exit(1)
			}
			var emailPtr *string
			if trimmedEmail := strings.TrimSpace(*email); trimmedEmail != "" {
				emailPtr = &trimmedEmail
			}
			admin, err := adminRepo.CreateAdmin(ctx, u, emailPtr, hash)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error creating admin: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Admin user '%s' created with ID %d.\n", admin.Username, admin.ID)

		case "reset-password":
			fs := flag.NewFlagSet("admin reset-password", flag.ExitOnError)
			username := fs.String("username", "", "Admin username")
			password := fs.String("password", "", "New admin password (min 8 chars)")
			_ = fs.Parse(os.Args[3:])

			u := strings.TrimSpace(*username)
			p := *password
			if u == "" || len(p) < 8 {
				fmt.Fprintln(os.Stderr, "Error: --username is required and --password must be at least 8 characters.")
				os.Exit(1)
			}
			hash, err := services.HashPassword(p)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error hashing password: %v\n", err)
				os.Exit(1)
			}
			if err := adminRepo.UpdatePasswordByUsername(ctx, u, hash); err != nil {
				fmt.Fprintf(os.Stderr, "Error resetting password: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Password for admin '%s' has been reset.\n", u)

		default:
			printUsage()
			os.Exit(1)
		}

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  baramail-cli migrate")
	fmt.Println("  baramail-cli cleanup")
	fmt.Println("  baramail-cli admin create --username <username> --password <password> [--email <email>]")
	fmt.Println("  baramail-cli admin reset-password --username <username> --password <password>")
}


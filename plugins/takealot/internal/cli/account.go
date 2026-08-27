package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/t12e/takealot-cli/internal/client"
)

func newAuthCommand() *cobra.Command {
	command := &cobra.Command{Use: "auth", Short: "Log in to or manage the Takealot mobile session"}
	command.AddCommand(newAuthLoginCommand(), newAuthStatusCommand(), newAuthLogoutCommand())
	return command
}

func newAuthLoginCommand() *cobra.Command {
	var email string
	var passwordStdin bool
	var trustDevice bool
	command := &cobra.Command{
		Use:   "login [--email <address>] [--password-stdin]",
		Short: "Log in using Takealot's Android authentication flow",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if !passwordStdin {
				return runBrowserLogin(command, email, trustDevice)
			}
			password, queuedOTP, err := readPasswordStdin(command)
			if err != nil {
				return err
			}
			result, err := client.NewAuthenticated().Login(command.Context(), email, password, func(_ context.Context) (string, bool, error) {
				return queuedOTP, trustDevice, nil
			})
			if err != nil {
				return err
			}
			if options.json {
				return writeJSON(command.OutOrStdout(), result)
			}
			fmt.Fprintf(command.OutOrStdout(), "Logged in to Takealot customer %s. Session saved in the OS keyring.\n", result.CustomerID)
			return nil
		},
	}
	command.Flags().StringVar(&email, "email", "", "pre-fill the Takealot account email on the local login page")
	command.Flags().BoolVar(&passwordStdin, "password-stdin", false, "read the password from stdin; the next line is used as the OTP if required")
	command.Flags().BoolVar(&trustDevice, "trust-device", true, "trust this device when Takealot requests OTP verification")
	return command
}

func newAuthStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether an authenticated session is saved",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			result, err := client.NewAuthenticated().Status()
			if err != nil {
				return err
			}
			if options.json {
				return writeJSON(command.OutOrStdout(), result)
			}
			if !result.Authenticated {
				fmt.Fprintln(command.OutOrStdout(), "Not logged in to Takealot.")
				return nil
			}
			fmt.Fprintf(command.OutOrStdout(), "Logged in as customer %s\nJWT expiry: %s\nRefresh-token expiry: %s\n", result.CustomerID, formatTime(result.JWTExpiresAt), formatTime(result.RefreshTokenExpiresAt))
			return nil
		},
	}
}

func newAuthLogoutCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the saved Takealot session from the OS keyring",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if err := client.NewAuthenticated().Logout(); err != nil {
				return fmt.Errorf("remove Takealot session: %w", err)
			}
			if options.json {
				return writeJSON(command.OutOrStdout(), map[string]any{"logged_out": true})
			}
			fmt.Fprintln(command.OutOrStdout(), "Takealot session removed from the OS keyring.")
			return nil
		},
	}
}

func newWishlistCommand() *cobra.Command {
	command := &cobra.Command{Use: "wishlist", Short: "View or explicitly modify Takealot wishlists"}
	command.AddCommand(newWishlistListCommand(), newWishlistItemsCommand(), newWishlistCreateCommand(), newWishlistRenameCommand(), newWishlistDeleteCommand(), newWishlistAddCommand(), newWishlistRemoveCommand())
	return command
}

func newWishlistListCommand() *cobra.Command {
	var page, pageSize int
	command := &cobra.Command{Use: "list", Short: "List saved wishlists", Args: cobra.NoArgs, RunE: func(command *cobra.Command, _ []string) error {
		result, err := client.NewAuthenticated().ListWishlists(command.Context(), page, pageSize)
		if err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), result)
		}
		if len(result.Wishlists) == 0 {
			fmt.Fprintln(command.OutOrStdout(), "No wishlists found.")
			return nil
		}
		for _, list := range result.Wishlists {
			fmt.Fprintf(command.OutOrStdout(), "%s\t%s\t%d item(s)\n", list.GroupID, list.Name, list.ItemCount)
		}
		return nil
	}}
	command.Flags().IntVar(&page, "page", 0, "zero-based page")
	command.Flags().IntVar(&pageSize, "page-size", 0, "items per page")
	return command
}

func newWishlistItemsCommand() *cobra.Command {
	var page, pageSize int
	command := &cobra.Command{Use: "items <group-id>", Short: "List items in a wishlist", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		result, err := client.NewAuthenticated().GetWishlistItems(command.Context(), args[0], page, pageSize)
		if err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), result)
		}
		fmt.Fprintf(command.OutOrStdout(), "%s (%s)\n", result.Name, result.GroupID)
		for _, item := range result.Items {
			fmt.Fprintf(command.OutOrStdout(), "%s\t%s\n", item.PLID, item.Title)
		}
		return nil
	}}
	command.Flags().IntVar(&page, "page", 0, "zero-based page")
	command.Flags().IntVar(&pageSize, "page-size", 0, "items per page")
	return command
}

func newWishlistCreateCommand() *cobra.Command {
	var confirm bool
	command := &cobra.Command{Use: "create <name> --confirm", Short: "Create a wishlist", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := requireConfirm(confirm); err != nil {
			return err
		}
		result, err := client.NewAuthenticated().CreateWishlist(command.Context(), args[0])
		if err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), result)
		}
		fmt.Fprintf(command.OutOrStdout(), "Created wishlist %s (%s).\n", result.Name, result.GroupID)
		return nil
	}}
	command.Flags().BoolVar(&confirm, "confirm", false, "confirm this state-changing action")
	return command
}

func newWishlistRenameCommand() *cobra.Command {
	var confirm bool
	command := &cobra.Command{Use: "rename <group-id> <name> --confirm", Short: "Rename a wishlist", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		if err := requireConfirm(confirm); err != nil {
			return err
		}
		result, err := client.NewAuthenticated().RenameWishlist(command.Context(), args[0], args[1])
		if err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), result)
		}
		fmt.Fprintf(command.OutOrStdout(), "Renamed wishlist %s to %s.\n", result.GroupID, result.Name)
		return nil
	}}
	command.Flags().BoolVar(&confirm, "confirm", false, "confirm this state-changing action")
	return command
}

func newWishlistDeleteCommand() *cobra.Command {
	var confirm bool
	command := &cobra.Command{Use: "delete <group-id> --confirm", Short: "Delete a wishlist", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := requireConfirm(confirm); err != nil {
			return err
		}
		if err := client.NewAuthenticated().DeleteWishlist(command.Context(), args[0]); err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), map[string]any{"action": "delete", "group_id": args[0], "completed": true})
		}
		fmt.Fprintf(command.OutOrStdout(), "Deleted wishlist %s.\n", args[0])
		return nil
	}}
	command.Flags().BoolVar(&confirm, "confirm", false, "confirm this state-changing action")
	return command
}

func newWishlistAddCommand() *cobra.Command {
	var confirm bool
	command := &cobra.Command{Use: "add <group-id> <plid-or-takealot-url> --confirm", Short: "Add a product to a wishlist", Args: cobra.ExactArgs(2), RunE: func(command *cobra.Command, args []string) error {
		if err := requireConfirm(confirm); err != nil {
			return err
		}
		result, err := client.NewAuthenticated().AddProductToWishlist(command.Context(), args[0], args[1])
		if err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), result)
		}
		fmt.Fprintf(command.OutOrStdout(), "Added %s to wishlist %s.\n", result.Product.Title, result.GroupID)
		return nil
	}}
	command.Flags().BoolVar(&confirm, "confirm", false, "confirm this state-changing action")
	return command
}

func newWishlistRemoveCommand() *cobra.Command {
	var confirm bool
	command := &cobra.Command{Use: "remove <plid-or-takealot-url> --confirm", Short: "Remove a product from all wishlists", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := requireConfirm(confirm); err != nil {
			return err
		}
		result, err := client.NewAuthenticated().RemoveProductFromWishlists(command.Context(), args[0])
		if err != nil {
			return err
		}
		if options.json {
			return writeJSON(command.OutOrStdout(), result)
		}
		fmt.Fprintf(command.OutOrStdout(), "Removed %s from all wishlists.\n", result.Product.Title)
		return nil
	}}
	command.Flags().BoolVar(&confirm, "confirm", false, "confirm this state-changing action")
	return command
}

func requireConfirm(confirm bool) error {
	if !confirm {
		return errors.New("this action changes Takealot state; repeat the command with --confirm after explicit user confirmation")
	}
	return nil
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return value.Local().Format(time.RFC3339)
}

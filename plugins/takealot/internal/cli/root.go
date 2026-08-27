package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/t12e/takealot-cli/internal/client"
	"github.com/t12e/takealot-cli/internal/models"
)

type rootOptions struct {
	json bool
}

var options rootOptions

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "takealot",
		Short:         "Research Takealot products and manage wishlists",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&options.json, "json", false, "output normalized JSON")
	root.AddCommand(newSearchCommand(), newProductCommand(), newVersionCommand(), newAuthCommand(), newWishlistCommand())
	return root
}

func newSearchCommand() *cobra.Command {
	var limit int
	command := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the Takealot catalogue",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if limit <= 0 {
				return errors.New("limit must be greater than zero")
			}
			result, err := client.New().Search(command.Context(), args[0], limit)
			if err != nil {
				return err
			}
			if options.json {
				return writeJSON(command.OutOrStdout(), result)
			}
			writeSearchTable(command.OutOrStdout(), result)
			return nil
		},
	}
	command.Flags().IntVar(&limit, "limit", 10, "maximum number of results")
	return command
}

func newProductCommand() *cobra.Command {
	product := &cobra.Command{Use: "product", Short: "Inspect products and reviews"}
	product.AddCommand(newProductGetCommand(), newProductImagesCommand(), newProductReviewsCommand())
	return product
}

func newProductGetCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "get <plid-or-takealot-url>",
		Short: "Fetch normalized product details",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := client.New().GetProduct(command.Context(), args[0])
			if err != nil {
				return err
			}
			if options.json {
				return writeJSON(command.OutOrStdout(), result)
			}
			writeProductDetails(command.OutOrStdout(), result)
			return nil
		},
	}
	return command
}

func newProductReviewsCommand() *cobra.Command {
	var rating int
	var sort string
	var page int
	var variant string
	command := &cobra.Command{
		Use:   "reviews <plid-or-takealot-url>",
		Short: "Fetch normalized product reviews",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			result, err := client.New().GetReviews(command.Context(), args[0], client.ReviewOptions{Rating: rating, Sort: sort, Page: page, Variant: variant})
			if err != nil {
				return err
			}
			if options.json {
				return writeJSON(command.OutOrStdout(), result)
			}
			writeReviews(command.OutOrStdout(), result)
			return nil
		},
	}
	command.Flags().IntVar(&rating, "rating", 0, "filter by rating from 1 to 5")
	command.Flags().StringVar(&sort, "sort", "helpful", "sort reviews by helpful or latest")
	command.Flags().IntVar(&page, "page", 0, "zero-based review page")
	command.Flags().StringVar(&variant, "variant", "", "filter by variant value, such as Black")
	return command
}

func newProductImagesCommand() *cobra.Command {
	var limit int
	var directory string
	command := &cobra.Command{
		Use:   "images <plid-or-takealot-url>",
		Short: "Download product images for local rendering",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if limit <= 0 {
				return errors.New("image limit must be greater than zero")
			}
			result, err := client.New().DownloadProductImages(command.Context(), args[0], client.ImageDownloadOptions{Limit: limit, Directory: directory})
			if err != nil {
				return err
			}
			if options.json {
				return writeJSON(command.OutOrStdout(), result)
			}
			writeProductImages(command.OutOrStdout(), result)
			return nil
		},
	}
	command.Flags().IntVar(&limit, "limit", 1, "number of gallery images to download, up to 10")
	command.Flags().StringVar(&directory, "dir", "", "image directory (default: ~/.takealot/images/<PLID>)")
	return command
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeSearchTable(writer io.Writer, result models.SearchResponse) {
	if result.Returned == 0 {
		fmt.Fprintln(writer, "No products found.")
		return
	}
	tab := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tab, "PLID\tPRODUCT\tPRICE\tRATING\tREVIEWS\tSTOCK")
	for _, product := range result.Results {
		fmt.Fprintf(tab, "%s\t%s\t%s\t%.1f\t%d\t%s\n", product.PLID, oneLine(product.Title), product.PriceDisplay, product.Rating.Average, product.Rating.Count, oneLine(product.Stock.Status))
	}
	_ = tab.Flush()
}

func writeProductDetails(writer io.Writer, product models.ProductDetails) {
	fmt.Fprintf(writer, "%s\n", product.Title)
	fmt.Fprintf(writer, "PLID: %s  Product ID: %d  TSIN: %d\n", product.PLID, product.ProductID, product.TSIN)
	fmt.Fprintf(writer, "Brand: %s  Price: %s  Rating: %.1f (%d reviews)\n", product.Brand, product.PriceDisplay, product.Rating.Average, product.Rating.Count)
	fmt.Fprintf(writer, "URL: %s\n", product.URL)
	if product.Stock.Status != "" {
		fmt.Fprintf(writer, "Stock: %s\n", product.Stock.Status)
	}
	if product.Description != "" {
		fmt.Fprintf(writer, "\nDescription:\n%s\n", product.Description)
	}
	if len(product.BulletPoints) > 0 {
		fmt.Fprintln(writer, "\nHighlights:")
		for _, point := range product.BulletPoints {
			fmt.Fprintf(writer, "- %s\n", point.Text)
		}
	}
	fmt.Fprintf(writer, "\nReview distribution: 1★ %d | 2★ %d | 3★ %d | 4★ %d | 5★ %d\n", product.Rating.Distribution.OneStar, product.Rating.Distribution.TwoStar, product.Rating.Distribution.ThreeStar, product.Rating.Distribution.FourStar, product.Rating.Distribution.FiveStar)
	if len(product.Gallery) > 0 {
		fmt.Fprintln(writer, "\nGallery:")
		for _, image := range product.Gallery {
			fmt.Fprintln(writer, image)
		}
	}
}

func writeReviews(writer io.Writer, result models.ReviewsResponse) {
	fmt.Fprintf(writer, "Page %d of %d (%d total reviews)\n", result.Page.CurrentPage, result.Page.TotalPages, result.Page.Total)
	if len(result.Reviews) == 0 {
		fmt.Fprintln(writer, "No reviews found for these filters.")
		return
	}
	for _, review := range result.Reviews {
		fmt.Fprintf(writer, "\n%d★  %s  (%d upvotes)\n%s\n", review.Rating, review.Date, review.Upvotes, strings.TrimSpace(review.Text))
		if len(review.VariantInfo) > 0 {
			fmt.Fprintf(writer, "Variant: %v\n", review.VariantInfo)
		}
	}
}

func writeProductImages(writer io.Writer, result models.ProductImagesResponse) {
	if len(result.Images) == 0 {
		fmt.Fprintf(writer, "No gallery images found for %s.\n", result.Title)
		return
	}
	fmt.Fprintf(writer, "Downloaded %d image(s) for %s\nDirectory: %s\n", len(result.Images), result.Title, result.Directory)
	for _, image := range result.Images {
		fmt.Fprintf(writer, "%d. %s\n", image.Index, image.LocalPath)
	}
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

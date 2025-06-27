package app

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/hydrz/lux/downloader"
	"github.com/hydrz/lux/extractors"
	"github.com/hydrz/lux/request"
	"github.com/hydrz/lux/utils"
)

// Name is the name of this app.
const Name = "lux"

// This value will be injected into the corresponding git tag value at build time using `-ldflags`.
var version = "v0.0.0"

// Global flags
var (
	// General flags
	debug      bool
	silent     bool
	info       bool
	jsonOutput bool

	// Authentication flags
	cookie    string
	userAgent string
	refer     string

	// Download options
	playlist       bool
	streamFormat   string
	audioOnly      bool
	file           string
	outputPath     string
	outputName     string
	fileNameLength uint
	caption        bool

	// Range options
	start uint
	end   uint
	items string

	// Performance options
	multiThread bool
	retry       uint
	chunkSize   uint
	thread      uint

	// Aria2 options
	aria2       bool
	aria2Token  string
	aria2Addr   string
	aria2Method string

	// Youku options
	youkuCcode    string
	youkuCkey     string
	youkuPassword string

	// Bilibili options
	episodeTitleOnly bool
)

// NewRootCmd creates the root command
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   Name,
		Short: "A fast and simple video downloader",
		Long: `lux is a fast and simple video downloader built in Go.
It supports downloading videos from various platforms including:
- YouTube, Bilibili, Douyin, Kuaishou
- Facebook, Instagram, Twitter
- Gaodun, Geekbang and many more...`,
		Version: version,
		RunE:    runDownload,
		Example: `  # Download a single video
  lux "https://www.youtube.com/watch?v=dQw4w9WgXcQ"

  # Download with custom output path
  lux -o ~/Downloads "https://example.com/video"

  # Download playlist
  lux -p "https://www.youtube.com/playlist?list=..."

  # Download with authentication
  lux -c "your-cookie" "https://protected-video.com"

  # Download from Gaodun
  GAODUN_AUTH_TOKEN="your-token" lux "https://gaodun.com/course?course_id=17244"`,
	}

	// Add flags
	addFlags(rootCmd)

	// Custom version template
	rootCmd.SetVersionTemplate(fmt.Sprintf(`
%s: version %s, A fast and simple video downloader.

`, color.New(color.FgCyan).Sprint(Name), color.New(color.FgBlue).Sprint(version)))

	return rootCmd
}

// addFlags adds all command line flags
func addFlags(cmd *cobra.Command) {
	// General flags
	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Debug mode")
	cmd.PersistentFlags().BoolVarP(&silent, "silent", "s", false, "Minimum outputs")
	cmd.PersistentFlags().BoolVarP(&info, "info", "i", false, "Information only")
	cmd.PersistentFlags().BoolVarP(&jsonOutput, "json", "j", false, "Print extracted JSON data")

	// Authentication flags
	cmd.PersistentFlags().StringVarP(&cookie, "cookie", "c", "", "Cookie")
	cmd.PersistentFlags().StringVarP(&userAgent, "user-agent", "u", "", "Use specified User-Agent")
	cmd.PersistentFlags().StringVarP(&refer, "refer", "r", "", "Use specified Referrer")

	// Download options
	cmd.PersistentFlags().BoolVarP(&playlist, "playlist", "p", false, "Download playlist")
	cmd.PersistentFlags().StringVarP(&streamFormat, "stream-format", "f", "", "Select specific stream to download")
	cmd.PersistentFlags().BoolVar(&audioOnly, "audio-only", false, "Download audio only at best quality")
	cmd.PersistentFlags().StringVarP(&file, "file", "F", "", "URLs file path")
	cmd.PersistentFlags().StringVarP(&outputPath, "output-path", "o", "", "Specify the output path")
	cmd.PersistentFlags().StringVarP(&outputName, "output-name", "O", "", "Specify the output file name")
	cmd.PersistentFlags().UintVar(&fileNameLength, "file-name-length", 255, "The maximum length of a file name, 0 means unlimited")
	cmd.PersistentFlags().BoolVarP(&caption, "caption", "C", false, "Download captions")

	// Range options
	cmd.PersistentFlags().UintVar(&start, "start", 1, "Define the starting item of a playlist or a file input")
	cmd.PersistentFlags().UintVar(&end, "end", 0, "Define the ending item of a playlist or a file input")
	cmd.PersistentFlags().StringVar(&items, "items", "", "Define wanted items from a file or playlist. Separated by commas like: 1,5,6,8-10")

	// Performance options
	cmd.PersistentFlags().BoolVarP(&multiThread, "multi-thread", "m", false, "Multiple threads to download single video")
	cmd.PersistentFlags().UintVar(&retry, "retry", 10, "How many times to retry when the download failed")
	cmd.PersistentFlags().UintVar(&chunkSize, "chunk-size", 1, "HTTP chunk size for downloading (in MB)")
	cmd.PersistentFlags().UintVarP(&thread, "thread", "n", 10, "The number of download thread (only works for multiple-parts video)")

	// Aria2 options
	cmd.PersistentFlags().BoolVar(&aria2, "aria2", false, "Use Aria2 RPC to download")
	cmd.PersistentFlags().StringVar(&aria2Token, "aria2-token", "", "Aria2 RPC Token")
	cmd.PersistentFlags().StringVar(&aria2Addr, "aria2-addr", "localhost:6800", "Aria2 Address")
	cmd.PersistentFlags().StringVar(&aria2Method, "aria2-method", "http", "Aria2 Method")

	// Youku options
	cmd.PersistentFlags().StringVar(&youkuCcode, "youku-ccode", "0502", "Youku ccode")
	cmd.PersistentFlags().StringVar(&youkuCkey, "youku-ckey", "7B19C0AB12633B22E7FE81271162026020570708D6CC189E4924503C49D243A0DE6CD84A766832C2C99898FC5ED31F3709BB3CDD82C96492E721BDD381735026", "Youku ckey")
	cmd.PersistentFlags().StringVar(&youkuPassword, "youku-password", "", "Youku password")

	// Bilibili options
	cmd.PersistentFlags().BoolVar(&episodeTitleOnly, "episode-title-only", false, "File name of each bilibili episode doesn't include the playlist title")

	// Add aliases for commonly used flags
	cmd.PersistentFlags().Lookup("audio-only").ShorthandDeprecated = "ao is deprecated, use --audio-only"
	cmd.PersistentFlags().Lookup("chunk-size").ShorthandDeprecated = "cs is deprecated, use --chunk-size"
	cmd.PersistentFlags().Lookup("youku-ccode").ShorthandDeprecated = "ccode is deprecated, use --youku-ccode"
	cmd.PersistentFlags().Lookup("youku-ckey").ShorthandDeprecated = "ckey is deprecated, use --youku-ckey"
	cmd.PersistentFlags().Lookup("youku-password").ShorthandDeprecated = "password is deprecated, use --youku-password"
	cmd.PersistentFlags().Lookup("episode-title-only").ShorthandDeprecated = "eto is deprecated, use --episode-title-only"
}

// runDownload is the main run function for the root command
func runDownload(cmd *cobra.Command, args []string) error {
	if debug {
		cmd.Printf(cmd.VersionTemplate())
	}

	// Handle file input
	var urls []string
	if file != "" {
		f, err := os.Open(file)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", file, err)
		}
		defer f.Close()

		fileItems := utils.ParseInputFile(f, items, int(start), int(end))
		urls = append(urls, fileItems...)
	}

	// Add command line arguments
	urls = append(urls, args...)

	if len(urls) < 1 {
		return fmt.Errorf("no URLs provided")
	}

	// Handle cookie
	finalCookie := cookie
	if finalCookie != "" {
		// If cookie is a file path, convert it to a string
		if _, fileErr := os.Stat(finalCookie); fileErr == nil {
			data, err := os.ReadFile(finalCookie)
			if err != nil {
				return fmt.Errorf("failed to read cookie file: %w", err)
			}
			finalCookie = strings.TrimSpace(string(data))
		}
	}

	// Set request options
	request.SetOptions(request.Options{
		RetryTimes: int(retry),
		Cookie:     finalCookie,
		UserAgent:  userAgent,
		Refer:      refer,
		Debug:      debug,
		Silent:     silent,
	})

	// Download each URL
	var hasError bool
	for _, videoURL := range urls {
		if err := downloadURL(videoURL); err != nil {
			fmt.Fprintf(
				color.Output,
				"Downloading %s error:\n",
				color.CyanString("%s", videoURL),
			)
			fmt.Printf("%+v\n", err)
			hasError = true
		}
	}

	if hasError {
		return fmt.Errorf("some downloads failed")
	}

	return nil
}

// downloadURL downloads a single URL
func downloadURL(videoURL string) error {
	data, err := extractors.Extract(videoURL, extractors.Options{
		Playlist:         playlist,
		Items:            items,
		ItemStart:        int(start),
		ItemEnd:          int(end),
		ThreadNumber:     int(thread),
		EpisodeTitleOnly: episodeTitleOnly,
		Cookie:           cookie,
		YoukuCcode:       youkuCcode,
		YoukuCkey:        youkuCkey,
		YoukuPassword:    youkuPassword,
	})
	if err != nil {
		return err
	}

	if jsonOutput {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "\t")
		e.SetEscapeHTML(false)
		if err := e.Encode(data); err != nil {
			return err
		}
		return nil
	}

	defaultDownloader := downloader.New(downloader.Options{
		Silent:         silent,
		InfoOnly:       info,
		Stream:         streamFormat,
		AudioOnly:      audioOnly,
		Refer:          refer,
		OutputPath:     outputPath,
		OutputName:     outputName,
		FileNameLength: int(fileNameLength),
		Caption:        caption,
		MultiThread:    multiThread,
		ThreadNumber:   int(thread),
		RetryTimes:     int(retry),
		ChunkSizeMB:    int(chunkSize),
		UseAria2RPC:    aria2,
		Aria2Token:     aria2Token,
		Aria2Method:    aria2Method,
		Aria2Addr:      aria2Addr,
	})

	var errors []error
	for _, item := range data {
		if item.Err != nil {
			errors = append(errors, item.Err)
			continue
		}
		if err = defaultDownloader.Download(item); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) != 0 {
		return errors[0]
	}
	return nil
}

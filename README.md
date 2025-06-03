# Series Downloader

[![Go](https://img.shields.io/badge/Go-1.23.4-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Maintenance](https://img.shields.io/badge/Maintained%3F-yes-green.svg)](https://github.com/yourusername/series_donwloader/graphs/commit-activity)
![GitHub Repo stars](https://img.shields.io/github/stars/icewizard98/series_downloader?style=flat)

A command-line tool to search, download, and manage anime or series episodes from various providers.

## Active providers
- [AnimeUnity](https://www.animeunity.so) Italian anime contents

## Features

- 🔍 Search for anime by title
- 📥 Download episodes individually or in batches
- 📚 Track watching history across different anime series
- 🔄 Resume watching from where you left off
- 🎬 Automatic playback of downloaded episodes
- 🧵 Multi-threaded downloads for better performance

## Installation

### Prerequisites

- Go 1.23.4 or higher

### From Source

```bash
# Clone the repository
git clone https://github.com/icewizard98/series_donwloader.git
cd series_donwloader

# Build the project
go build -o series_donwloader
```

## Usage

### Basic Usage

```bash
./series_donwloader --title "Naruto" --user "username"
```

### Command Line Arguments

- `--title`: The anime title to search for
- `--user`: The user profile to use (loads from `username.env`)

### Environment Variables

Create a `.env` file or a `username.env` file with the following variables:

```
USER_ROOT_DIR=/path/to/anime/directory
DOWNLOAD_NEXT_EPISODES=3  # Number of episodes to download in advance
```

### Example Workflow

1. Run the program with an anime title
2. Select the anime from the search results
3. If you've watched episodes before, you'll be asked if you want to continue
4. Otherwise, select which episode to watch
5. The program will download the selected episode and any additional episodes based on your configuration
6. Your watching history is automatically saved

## Project Structure

```
series_donwloader/
├── models/
│   ├── User.go             # User model and history management
│   ├── animeunity/         # AnimeUnity API integration
│   │   └── Animeunity.go   # AnimeUnity client implementation
├── utils/
│   └── routinepoll/        # Thread pool for concurrent downloads
├── init.go                 # Main application entry point
├── go.mod                  # Go module definition
└── README.md               # This file
```

## Configuration

### User Profiles

You can create multiple user profiles by creating different `.env` files. For example, if you have a user named "alice", create a file named `alice.env` with the appropriate configuration.

### Download Directory

Set the `USER_ROOT_DIR` environment variable to specify where downloaded episodes should be stored.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [GoQuery](https://github.com/PuerkitoBio/goquery) for HTML parsing
- [godotenv](https://github.com/joho/godotenv) for environment variable management

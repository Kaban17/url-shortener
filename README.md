
# URL Shortener Service

A lightweight URL shortening service written in Go with SQLite storage.

## Features

- Shorten long URLs to compact 6-8 character codes
- Redirect short URLs to original destinations
- Persistent storage using SQLite
- Simple REST API
- Web interface for URL shortening

## Architecture
├── cmd/ # Main application
├── internal/ # Private application code
│ ├── handler/ # HTTP handlers
│ └── storage/ # Database layer
├── pkg/ # Reusable components
│ └── urlshortener/ # URL shortening logic
└── static/ # Static files

## Installation

### Prerequisites

- Go 1.21+
- SQLite3

### Build and Run

```bash
# Clone the repository
git clone https://github.com/yourusername/url-shortener.git
cd url-shortener

# Build and run
go build -o shortener ./cmd/url-shortener
./shortener

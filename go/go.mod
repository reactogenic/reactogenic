module github.com/reactogenic/reactogenic/go

go 1.27.0

require (
	github.com/andybalholm/cascadia v1.3.5
	github.com/evanw/esbuild v0.28.2
	github.com/microsoft/TypeScript/tsc v0.0.0-00010101000000-000000000000
	golang.org/x/net v0.59.0
	modernc.org/quickjs v0.25.0
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.20.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/mackerelio/go-osstat v0.2.8 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	modernc.org/libc v1.77.1 // indirect
	modernc.org/libquickjs v0.13.1 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.12.1 // indirect
)

replace github.com/microsoft/TypeScript/tsc => ./third_party/tsgo

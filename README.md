# aread

This is a Go server that downloads and simplifies web pages so that they can be
read later. Simplification happens via [Mercury Parser], and the resulting pages
are also converted to Amazon's `.mobi` format using [KindleGen] and sent to a
Kindle device. One of the design goals was to use no JavaScript in the web
interface.

[Mercury Parser]: https://github.com/postlight/mercury-parser
[KindleGen]: https://www.amazon.com/gp/feature.html?ie=UTF8&docId=1000765211

## Configuration

Install [Mercury Parser] and [KindleGen].

Create a directory (e.g. `/var/lib/aread`) and create a `config.json` file
corresponding to the `Config` struct in [common/config.go](./common/config.go).
Also create `url_patterns.json`, `bad_content.json`, and `hidden_tags.json` as
described in the same struct.

Then run a command like `aread --config=/var/lib/aread/config.json`.

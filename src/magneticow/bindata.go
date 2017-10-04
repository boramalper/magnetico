package main

import (
	"fmt"
	"io/ioutil"
)

// bindata_read reads the given file from disk. It returns
// an error on failure.
func bindata_read(path, name string) ([]byte, error) {
	buf, err := ioutil.ReadFile(path)
	if err != nil {
		err = fmt.Errorf("Error reading asset %s at %s: %v", name, path, err)
	}
	return buf, err
}


// templates_torrent_html reads file data from disk.
// It panics if something went wrong in the process.
func templates_torrent_html() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/templates/torrent.html",
		"templates/torrent.html",
	)
}

// templates_feed_xml reads file data from disk.
// It panics if something went wrong in the process.
func templates_feed_xml() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/templates/feed.xml",
		"templates/feed.xml",
	)
}

// templates_homepage_html reads file data from disk.
// It panics if something went wrong in the process.
func templates_homepage_html() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/templates/homepage.html",
		"templates/homepage.html",
	)
}

// templates_statistics_html reads file data from disk.
// It panics if something went wrong in the process.
func templates_statistics_html() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/templates/statistics.html",
		"templates/statistics.html",
	)
}

// templates_torrents_html reads file data from disk.
// It panics if something went wrong in the process.
func templates_torrents_html() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/templates/torrents.html",
		"templates/torrents.html",
	)
}

// static_scripts_plotly_v1_26_1_min_js reads file data from disk.
// It panics if something went wrong in the process.
func static_scripts_plotly_v1_26_1_min_js() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/scripts/plotly-v1.26.1.min.js",
		"static/scripts/plotly-v1.26.1.min.js",
	)
}

// static_scripts_statistics_js reads file data from disk.
// It panics if something went wrong in the process.
func static_scripts_statistics_js() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/scripts/statistics.js",
		"static/scripts/statistics.js",
	)
}

// static_scripts_torrent_js reads file data from disk.
// It panics if something went wrong in the process.
func static_scripts_torrent_js() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/scripts/torrent.js",
		"static/scripts/torrent.js",
	)
}

// static_styles_reset_css reads file data from disk.
// It panics if something went wrong in the process.
func static_styles_reset_css() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/styles/reset.css",
		"static/styles/reset.css",
	)
}

// static_styles_statistics_css reads file data from disk.
// It panics if something went wrong in the process.
func static_styles_statistics_css() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/styles/statistics.css",
		"static/styles/statistics.css",
	)
}

// static_styles_torrent_css reads file data from disk.
// It panics if something went wrong in the process.
func static_styles_torrent_css() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/styles/torrent.css",
		"static/styles/torrent.css",
	)
}

// static_styles_torrents_css reads file data from disk.
// It panics if something went wrong in the process.
func static_styles_torrents_css() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/styles/torrents.css",
		"static/styles/torrents.css",
	)
}

// static_styles_homepage_css reads file data from disk.
// It panics if something went wrong in the process.
func static_styles_homepage_css() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/styles/homepage.css",
		"static/styles/homepage.css",
	)
}

// static_styles_essential_css reads file data from disk.
// It panics if something went wrong in the process.
func static_styles_essential_css() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/styles/essential.css",
		"static/styles/essential.css",
	)
}

// static_assets_magnet_gif reads file data from disk.
// It panics if something went wrong in the process.
func static_assets_magnet_gif() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/assets/magnet.gif",
		"static/assets/magnet.gif",
	)
}

// static_assets_feed_png reads file data from disk.
// It panics if something went wrong in the process.
func static_assets_feed_png() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/assets/feed.png",
		"static/assets/feed.png",
	)
}

// static_fonts_notomono_license_ofl_txt reads file data from disk.
// It panics if something went wrong in the process.
func static_fonts_notomono_license_ofl_txt() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/fonts/NotoMono/LICENSE_OFL.txt",
		"static/fonts/NotoMono/LICENSE_OFL.txt",
	)
}

// static_fonts_notomono_regular_ttf reads file data from disk.
// It panics if something went wrong in the process.
func static_fonts_notomono_regular_ttf() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/fonts/NotoMono/Regular.ttf",
		"static/fonts/NotoMono/Regular.ttf",
	)
}

// static_fonts_notosansui_license_ofl_txt reads file data from disk.
// It panics if something went wrong in the process.
func static_fonts_notosansui_license_ofl_txt() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/fonts/NotoSansUI/LICENSE_OFL.txt",
		"static/fonts/NotoSansUI/LICENSE_OFL.txt",
	)
}

// static_fonts_notosansui_bold_ttf reads file data from disk.
// It panics if something went wrong in the process.
func static_fonts_notosansui_bold_ttf() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/fonts/NotoSansUI/Bold.ttf",
		"static/fonts/NotoSansUI/Bold.ttf",
	)
}

// static_fonts_notosansui_bolditalic_ttf reads file data from disk.
// It panics if something went wrong in the process.
func static_fonts_notosansui_bolditalic_ttf() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/fonts/NotoSansUI/BoldItalic.ttf",
		"static/fonts/NotoSansUI/BoldItalic.ttf",
	)
}

// static_fonts_notosansui_italic_ttf reads file data from disk.
// It panics if something went wrong in the process.
func static_fonts_notosansui_italic_ttf() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/fonts/NotoSansUI/Italic.ttf",
		"static/fonts/NotoSansUI/Italic.ttf",
	)
}

// static_fonts_notosansui_regular_ttf reads file data from disk.
// It panics if something went wrong in the process.
func static_fonts_notosansui_regular_ttf() ([]byte, error) {
	return bindata_read(
		"/home/bora/labs/magnetico/src/magneticow/data/static/fonts/NotoSansUI/Regular.ttf",
		"static/fonts/NotoSansUI/Regular.ttf",
	)
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	if f, ok := _bindata[name]; ok {
		return f()
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string] func() ([]byte, error) {
	"templates/torrent.html": templates_torrent_html,
	"templates/feed.xml": templates_feed_xml,
	"templates/homepage.html": templates_homepage_html,
	"templates/statistics.html": templates_statistics_html,
	"templates/torrents.html": templates_torrents_html,
	"static/scripts/plotly-v1.26.1.min.js": static_scripts_plotly_v1_26_1_min_js,
	"static/scripts/statistics.js": static_scripts_statistics_js,
	"static/scripts/torrent.js": static_scripts_torrent_js,
	"static/styles/reset.css": static_styles_reset_css,
	"static/styles/statistics.css": static_styles_statistics_css,
	"static/styles/torrent.css": static_styles_torrent_css,
	"static/styles/torrents.css": static_styles_torrents_css,
	"static/styles/homepage.css": static_styles_homepage_css,
	"static/styles/essential.css": static_styles_essential_css,
	"static/assets/magnet.gif": static_assets_magnet_gif,
	"static/assets/feed.png": static_assets_feed_png,
	"static/fonts/NotoMono/LICENSE_OFL.txt": static_fonts_notomono_license_ofl_txt,
	"static/fonts/NotoMono/Regular.ttf": static_fonts_notomono_regular_ttf,
	"static/fonts/NotoSansUI/LICENSE_OFL.txt": static_fonts_notosansui_license_ofl_txt,
	"static/fonts/NotoSansUI/Bold.ttf": static_fonts_notosansui_bold_ttf,
	"static/fonts/NotoSansUI/BoldItalic.ttf": static_fonts_notosansui_bolditalic_ttf,
	"static/fonts/NotoSansUI/Italic.ttf": static_fonts_notosansui_italic_ttf,
	"static/fonts/NotoSansUI/Regular.ttf": static_fonts_notosansui_regular_ttf,

}

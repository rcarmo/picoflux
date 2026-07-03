// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mediaproxy // import "miniflux.app/v2/internal/mediaproxy"

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/reader/sanitizer"

	"github.com/PuerkitoBio/goquery"
)

// EInkImageRelativeURL returns a signed, local URL that serves a server-side
// e-ink adapted rendition of an image. The original URL is embedded but HMAC
// signed with the media-proxy private key, matching the existing media proxy's
// trust model.
func EInkImageRelativeURL(mediaURL string) string {
	if mediaURL == "" {
		return ""
	}

	mediaURLBytes := []byte(mediaURL)
	mac := hmac.New(sha256.New, config.Opts.MediaProxyPrivateKey())
	mac.Write(mediaURLBytes)
	digest := mac.Sum(nil)

	return fmt.Sprintf("%s/eink-image/%s/%s", config.Opts.BasePath(), base64.URLEncoding.EncodeToString(digest), base64.URLEncoding.EncodeToString(mediaURLBytes))
}

// RewriteDocumentWithEInkImageProxyURL rewrites absolute HTTP(S) images in an
// entry document to the local server-side e-ink image adaptation endpoint. It
// leaves stored entry content untouched; rewriting happens only at render time.
func RewriteDocumentWithEInkImageProxyURL(htmlDocument string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlDocument))
	if err != nil {
		return htmlDocument
	}

	doc.Find("img, picture source").Each(func(i int, img *goquery.Selection) {
		if srcAttrValue, ok := img.Attr("src"); ok && shouldAdaptImageURL(srcAttrValue) {
			img.SetAttr("data-original-src", srcAttrValue)
			img.SetAttr("src", EInkImageRelativeURL(srcAttrValue))
		}

		if srcsetAttrValue, ok := img.Attr("srcset"); ok {
			rewriteEInkSourceSet(img, srcsetAttrValue)
		}
	})

	doc.Find("video").Each(func(i int, video *goquery.Selection) {
		if posterAttrValue, ok := video.Attr("poster"); ok && shouldAdaptImageURL(posterAttrValue) {
			video.SetAttr("data-original-poster", posterAttrValue)
			video.SetAttr("poster", EInkImageRelativeURL(posterAttrValue))
		}
	})

	output, err := doc.FindMatcher(goquery.Single("body")).Html()
	if err != nil {
		return htmlDocument
	}
	return output
}

func rewriteEInkSourceSet(element *goquery.Selection, srcsetAttrValue string) {
	imageCandidates := sanitizer.ParseSrcSetAttribute(srcsetAttrValue)
	changed := false
	for _, imageCandidate := range imageCandidates {
		if shouldAdaptImageURL(imageCandidate.ImageURL) {
			imageCandidate.ImageURL = EInkImageRelativeURL(imageCandidate.ImageURL)
			changed = true
		}
	}
	if changed {
		element.SetAttr("data-original-srcset", srcsetAttrValue)
		element.SetAttr("srcset", imageCandidates.String())
	}
}

func shouldAdaptImageURL(mediaURL string) bool {
	parsedURL, err := url.Parse(mediaURL)
	if err != nil || !parsedURL.IsAbs() || parsedURL.Host == "" {
		return false
	}
	return strings.EqualFold(parsedURL.Scheme, "http") || strings.EqualFold(parsedURL.Scheme, "https")
}

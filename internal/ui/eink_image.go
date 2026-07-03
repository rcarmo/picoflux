// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/crypto"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/reader/fetcher"
	"miniflux.app/v2/internal/reader/rewrite"
)

const (
	einkImageMaxWidth  = 900
	einkImageMaxHeight = 1400
	einkImageJPEGQual  = 72
)

func (h *handler) einkImage(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("If-None-Match") != "" {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	encodedURL := request.RouteStringParam(r, "encodedURL")
	if encodedURL == "" {
		response.HTMLBadRequest(w, r, errors.New("no URL provided"))
		return
	}

	encodedDigest := request.RouteStringParam(r, "encodedDigest")
	decodedDigest, err := base64.URLEncoding.DecodeString(encodedDigest)
	if err != nil {
		response.HTMLBadRequest(w, r, errors.New("unable to decode this digest"))
		return
	}

	decodedURL, err := base64.URLEncoding.DecodeString(encodedURL)
	if err != nil {
		response.HTMLBadRequest(w, r, errors.New("unable to decode this URL"))
		return
	}

	mac := hmac.New(sha256.New, config.Opts.MediaProxyPrivateKey())
	mac.Write(decodedURL)
	if !hmac.Equal(decodedDigest, mac.Sum(nil)) {
		response.HTMLForbidden(w, r)
		return
	}

	parsedMediaURL, err := url.Parse(string(decodedURL))
	if err != nil || !parsedMediaURL.IsAbs() || parsedMediaURL.Host == "" || (parsedMediaURL.Scheme != "http" && parsedMediaURL.Scheme != "https") {
		response.HTMLBadRequest(w, r, errors.New("invalid URL provided"))
		return
	}

	mediaURL := string(decodedURL)
	requestBuilder := fetcher.NewRequestBuilder().
		WithTimeout(config.Opts.MediaProxyHTTPClientTimeout()).
		WithoutCompression()
	if referer := rewrite.GetRefererForURL(mediaURL); referer != "" {
		requestBuilder = requestBuilder.WithHeader("Referer", referer)
	}
	if ua := r.Header.Get("User-Agent"); ua != "" {
		requestBuilder = requestBuilder.WithHeader("User-Agent", ua)
	}

	resp, err := requestBuilder.ExecuteRequest(mediaURL)
	if err != nil {
		if errors.Is(err, fetcher.ErrPrivateNetworkHost) || errors.Is(err, fetcher.ErrHostnameResolution) {
			slog.Warn("EInkImage: Refused remote resource", slog.String("media_url", mediaURL), slog.Any("error", err))
			response.HTMLForbidden(w, r)
			return
		}
		slog.Warn("EInkImage: Unable to fetch remote resource", slog.String("media_url", mediaURL), slog.Any("error", err))
		http.Redirect(w, r, mediaURL, http.StatusFound)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Warn("EInkImage: Unexpected response status code", slog.String("media_url", mediaURL), slog.Int("status_code", resp.StatusCode))
		http.Redirect(w, r, mediaURL, http.StatusFound)
		return
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		slog.Warn("EInkImage: Unable to decode image, falling back to origin", slog.String("media_url", mediaURL), slog.Any("error", err))
		http.Redirect(w, r, mediaURL, http.StatusFound)
		return
	}

	adapted := adaptImageForEInk(img)
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, adapted, &jpeg.Options{Quality: einkImageJPEGQual}); err != nil {
		slog.Warn("EInkImage: Unable to encode adapted image", slog.String("media_url", mediaURL), slog.Any("error", err))
		http.Redirect(w, r, mediaURL, http.StatusFound)
		return
	}

	etag := crypto.HashFromBytes(append([]byte("eink:"), decodedURL...))
	response.NewBuilder(w, r).WithCaching(etag, 72*time.Hour, func(b *response.Builder) {
		b.WithStatus(http.StatusOK)
		b.WithHeader("Content-Security-Policy", response.ContentSecurityPolicyForUntrustedContent)
		b.WithHeader("Content-Type", "image/jpeg")
		b.WithBodyAsBytes(encoded.Bytes())
		b.WithoutCompression()
		b.Write()
	})
}

func adaptImageForEInk(src image.Image) image.Image {
	bounds := src.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width <= 0 || height <= 0 {
		return src
	}

	targetWidth, targetHeight := width, height
	if targetWidth > einkImageMaxWidth {
		targetWidth = einkImageMaxWidth
		targetHeight = height * targetWidth / width
	}
	if targetHeight > einkImageMaxHeight {
		targetHeight = einkImageMaxHeight
		targetWidth = width * targetHeight / height
	}
	if targetWidth < 1 {
		targetWidth = 1
	}
	if targetHeight < 1 {
		targetHeight = 1
	}

	resized := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(resized, resized.Bounds(), src, bounds, draw.Over, nil)

	gray := image.NewGray(resized.Bounds())
	for y := resized.Bounds().Min.Y; y < resized.Bounds().Max.Y; y++ {
		for x := resized.Bounds().Min.X; x < resized.Bounds().Max.X; x++ {
			r, g, b, _ := resized.At(x, y).RGBA()
			// Luma in 8-bit space.
			luma := int((299*(r>>8) + 587*(g>>8) + 114*(b>>8)) / 1000)
			// Mild contrast stretch around mid-gray, tuned for e-ink readability.
			luma = 128 + (luma-128)*145/100
			if luma < 0 {
				luma = 0
			} else if luma > 255 {
				luma = 255
			}
			gray.SetGray(x, y, color.Gray{Y: uint8(luma)})
		}
	}
	return gray
}

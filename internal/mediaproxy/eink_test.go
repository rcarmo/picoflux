// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mediaproxy // import "miniflux.app/v2/internal/mediaproxy"

import (
	"os"
	"strings"
	"testing"

	"miniflux.app/v2/internal/config"
)

func TestRewriteDocumentWithEInkImageProxyURL(t *testing.T) {
	os.Clearenv()
	os.Setenv("MEDIA_PROXY_PRIVATE_KEY", "test")

	var err error
	parser := config.NewConfigParser()
	config.Opts, err = parser.ParseEnvironmentVariables()
	if err != nil {
		t.Fatalf(`Config parsing failure: %v`, err)
	}

	input := `<p><img src="https://example.org/image.jpg" srcset="https://example.org/image-small.jpg 1x, https://example.org/image-large.jpg 2x"><img src="/local.png"></p>`
	output := RewriteDocumentWithEInkImageProxyURL(input)

	if !strings.Contains(output, `src="/eink-image/`) {
		t.Fatalf("expected img src to be rewritten to eink-image endpoint, got %q", output)
	}
	if !strings.Contains(output, `data-original-src="https://example.org/image.jpg"`) {
		t.Fatalf("expected original src to be preserved, got %q", output)
	}
	if !strings.Contains(output, `data-original-srcset="https://example.org/image-small.jpg 1x, https://example.org/image-large.jpg 2x"`) {
		t.Fatalf("expected original srcset to be preserved, got %q", output)
	}
	if strings.Contains(output, `/eink-image/`) && !strings.Contains(output, `src="/local.png"`) {
		t.Fatalf("expected relative/local images to be left alone, got %q", output)
	}
}

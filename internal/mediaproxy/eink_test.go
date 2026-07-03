// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package mediaproxy // import "miniflux.app/v2/internal/mediaproxy"

import (
	"strings"
	"testing"
)

func TestRewriteDocumentWithEInkImageProxyURL(t *testing.T) {
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

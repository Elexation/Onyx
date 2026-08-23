package handler

import (
	"strings"
	"testing"
)

func TestRewriteShareHLS_StripsInternalPath(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		token        string
		internalPath string
		wantContains []string
		wantAbsent   []string
	}{
		{
			name: "master playlist strips directory structure",
			input: "#EXTM3U\n" +
				"#EXT-X-VERSION:7\n" +
				"#EXT-X-STREAM-INF:BANDWIDTH=2000000,RESOLUTION=1920x1080\n" +
				"/api/stream/playlist/0/private/videos/movie.mkv\n" +
				"#EXT-X-STREAM-INF:BANDWIDTH=1000000,RESOLUTION=1280x720\n" +
				"/api/stream/playlist/1/private/videos/movie.mkv\n",
			token:        "abc123",
			internalPath: "/private/videos/movie.mkv",
			wantContains: []string{
				"/api/public/s/abc123/stream/playlist/0/movie.mkv",
				"/api/public/s/abc123/stream/playlist/1/movie.mkv",
			},
			wantAbsent: []string{
				"/private/videos/",
				"/api/stream/playlist",
			},
		},
		{
			name: "variant playlist strips from init and segment URLs",
			input: "#EXTM3U\n" +
				"#EXT-X-MAP:URI=\"/api/stream/init/0/deep/nested/dir/file.mp4\"\n" +
				"#EXTINF:6.006000,\n" +
				"/api/stream/segment/0/0/deep/nested/dir/file.mp4\n" +
				"#EXTINF:6.006000,\n" +
				"/api/stream/segment/0/1/deep/nested/dir/file.mp4\n",
			token:        "def456",
			internalPath: "/deep/nested/dir/file.mp4",
			wantContains: []string{
				"/api/public/s/def456/stream/init/0/file.mp4",
				"/api/public/s/def456/stream/segment/0/0/file.mp4",
				"/api/public/s/def456/stream/segment/0/1/file.mp4",
			},
			wantAbsent: []string{
				"/deep/nested/dir/",
			},
		},
		{
			name: "path with spaces is URL-encoded correctly",
			input: "#EXTM3U\n" +
				"#EXT-X-STREAM-INF:BANDWIDTH=2000000\n" +
				"/api/stream/playlist/0/my%20folder/my%20video.mkv\n",
			token:        "tok1",
			internalPath: "/my folder/my video.mkv",
			wantContains: []string{
				"/api/public/s/tok1/stream/playlist/0/my%20video.mkv",
			},
			wantAbsent: []string{
				"my%20folder",
			},
		},
		{
			name: "root-level file has no parent to strip",
			input: "#EXTM3U\n" +
				"#EXT-X-STREAM-INF:BANDWIDTH=2000000\n" +
				"/api/stream/playlist/0/movie.mkv\n",
			token:        "tok2",
			internalPath: "/movie.mkv",
			wantContains: []string{
				"/api/public/s/tok2/stream/playlist/0/movie.mkv",
			},
			wantAbsent: []string{
				"/api/stream/playlist",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := string(rewriteShareHLS([]byte(tt.input), tt.token, tt.internalPath))
			for _, want := range tt.wantContains {
				if !strings.Contains(result, want) {
					t.Errorf("expected result to contain %q\ngot:\n%s", want, result)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(result, absent) {
					t.Errorf("expected result NOT to contain %q\ngot:\n%s", absent, result)
				}
			}
		})
	}
}

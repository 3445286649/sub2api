package service

import "testing"

func TestNormalizeVersionTag(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "plain semver",
			in:   "0.1.125",
			want: "0.1.125",
		},
		{
			name: "tag prefix",
			in:   "v0.1.125",
			want: "0.1.125",
		},
		{
			name: "local build suffix",
			in:   "0.1.125-loucer-20260508-075819",
			want: "0.1.125",
		},
		{
			name: "metadata suffix",
			in:   "0.1.125+loucer",
			want: "0.1.125",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeVersionTag(tt.in); got != tt.want {
				t.Fatalf("normalizeVersionTag(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCompareVersionsIgnoresLocalBuildSuffix(t *testing.T) {
	if got := compareVersions("0.1.125-loucer-20260508-075819", "0.1.125"); got != 0 {
		t.Fatalf("compareVersions with local suffix = %d, want 0", got)
	}
}

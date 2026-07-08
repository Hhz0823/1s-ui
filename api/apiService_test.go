package api

import "testing"

func TestSysctlListHas(t *testing.T) {
	tests := []struct {
		name  string
		list  string
		value string
		want  bool
	}{
		{name: "exact token", list: "reno cubic bbr", value: "bbr", want: true},
		{name: "missing token", list: "reno cubic bbr", value: "bbr3", want: false},
		{name: "substring is not token", list: "reno cubic bbr3", value: "bbr", want: false},
		{name: "extra whitespace", list: "  reno\tcubic\nbbr2plus  ", value: "bbr2plus", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sysctlListHas(tt.list, tt.value); got != tt.want {
				t.Fatalf("sysctlListHas(%q, %q) = %v, want %v", tt.list, tt.value, got, tt.want)
			}
		})
	}
}

package server

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestJoinURL(t *testing.T) {
	tests := []struct {
		base   string
		paths  []string
		expect string
	}{
		{"./files", []string{"foo", "bar.jpg"}, "files/foo/bar.jpg"},
		{"./files", []string{"./foo/", "./bar.jpg/"}, "files/foo/bar.jpg"},
		{"./files/", []string{"./foo/", "./bar.jpg///"}, "files/foo/bar.jpg"},
		{"files", []string{"foo", "bar", "baz.txt"}, "files/foo/bar/baz.txt"},
		{"./files/", []string{"/foo/", "/bar/", "/baz.txt"}, "files/foo/bar/baz.txt"},
		{"files", []string{"foo"}, "files/foo"},
		{"files/", []string{}, "files"},
		{"./", []string{"foo", "bar"}, "foo/bar"},
		{"", []string{"foo", "bar"}, "foo/bar"},
		{"/", []string{"foo", "bar"}, "foo/bar"},
		{"./files", []string{""}, "files"},
		{"./files", []string{"."}, "files"},
		{"./files", []string{".."}, "files"},
		{"./files", []string{"/"}, "files"},
		{"./files", []string{"./"}, "files"},
		{"./files", []string{"../"}, "files"},
		{"./files", []string{"foo/", "/bar/", "/baz/"}, "files/foo/bar/baz"},
		{"./files", []string{"foo//bar//baz"}, "files/foo/bar/baz"},
		{"./files", []string{"foo", "", "bar.jpg"}, "files/foo/bar.jpg"},
		{"./files", []string{".", "foo", ".", "bar.jpg"}, "files/foo/bar.jpg"},
	}

	for _, tt := range tests {
		Convey("JoinURL("+tt.base+", "+fmt.Sprintf("%q", tt.paths)+") should be "+tt.expect, t, func() {
			got := JoinURL(tt.base, tt.paths...)

			So(got, ShouldEqual, tt.expect)
		},
		)
	}
}

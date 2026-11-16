package bilibili

import "testing"

func TestPostProcess(t *testing.T) {
	video := SearchResult{Title: `这才是真正的<em class="keyword">机甲大师</em>！！！`}
	video.postprocess()
	if got, want := video.GetTitle(), "这才是真正的机甲大师！！！"; got != want {
		t.Fatalf("title %q, want %q", got, want)
	}
}

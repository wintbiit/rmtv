package bilibili

import "testing"

func TestSearchVideo(t *testing.T) {
	client := NewClient()

	videos, err := client.SearchVideos("RoboMaster")
	if err != nil {
		t.Fatalf("failed to search videos: %v", err)
	}

	t.Logf("Found %d videos, \n%+v", len(videos), videos)
}

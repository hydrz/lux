package gaodun

import (
	"fmt"
	"testing"
)

func TestClient(t *testing.T) {
	client := NewClient()
	gStudyGradations, err := client.GStudy("33795")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	fmt.Printf("Glive Syllabus: %+v\n", gStudyGradations)

	gStudySyllabus, err := client.GStudySyllabus("33795", "49752")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	fmt.Printf("Glive Syllabus: %+v\n", *gStudySyllabus)

	epStudyGradations, err := client.EpStudy("17244")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	fmt.Printf("Ep Gradation: %+v\n", epStudyGradations)

	epStudySyllabus, err := client.EpStudySyllabus("17244", "17785")
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	fmt.Printf("Ep Syllabus: %+v\n", epStudySyllabus)

	videoResource, err := client.VideoResource("628hgv1x0k1ffvYn", "SD", 0)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	fmt.Printf("Video Resource: %+v\n", videoResource)

}

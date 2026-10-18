package services

import (
	"testing"

	"github.com/example/ozmap-user-manager/internal/models"
)

func TestCreationProgressSnapshotsResultsOnlyWhenComplete(t *testing.T) {
	service := &ImportService{}
	job := &creationJob{progress: CreationProgress{
		ImportID: "import-1",
		Total:    2,
		Status:   "processing",
		Results:  make([]models.UserResult, 0, 2),
	}}
	service.creationJobs.Store(job.progress.ImportID, job)

	service.recordCreationResult(job.progress.ImportID, models.UserResult{
		Email:  "user@example.com",
		Status: "created",
	})
	progress, ok := service.CreationProgress(job.progress.ImportID)
	if !ok {
		t.Fatal("CreationProgress() did not find job")
	}
	if progress.Processed != 1 || progress.Created != 1 || progress.Failed != 0 {
		t.Fatalf("in-progress counts = %#v", progress)
	}
	if progress.Results != nil {
		t.Fatal("CreationProgress() exposed incomplete results")
	}

	service.recordCreationResult(job.progress.ImportID, models.UserResult{
		Email:  "other@example.com",
		Status: "error",
	})
	job.mu.Lock()
	job.progress.Status = "completed"
	job.progress.Complete = true
	job.mu.Unlock()

	progress, ok = service.CreationProgress(job.progress.ImportID)
	if !ok {
		t.Fatal("CreationProgress() did not find completed job")
	}
	if progress.Processed != 2 || progress.Created != 1 || progress.Failed != 1 || !progress.Complete {
		t.Fatalf("completed counts = %#v", progress)
	}
	if len(progress.Results) != 2 {
		t.Fatalf("completed results = %d, want 2", len(progress.Results))
	}
}

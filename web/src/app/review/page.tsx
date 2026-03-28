import { ReviewShell } from "@/components/review-shell";
import { getMediaLibraryStatus, getReviewJob, listJobs } from "@/lib/api";

export default async function ReviewPage() {
  const result = await listJobs({
    status: "reviewing",
    page: 1,
    pageSize: 200,
  });
  const jobs = result.items;
  let initialMediaStatus = null;
  const initialScrapeData = jobs.length > 0 ? await getReviewJob(jobs[0].id) : null;
  try {
    initialMediaStatus = await getMediaLibraryStatus();
  } catch {
    initialMediaStatus = null;
  }

  return (
    <div style={{ height: "100%" }}>
      <ReviewShell jobs={jobs} initialScrapeData={initialScrapeData} initialMediaStatus={initialMediaStatus} />
    </div>
  );
}

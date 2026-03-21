import { ReviewShell } from "@/components/review-shell";
import { getReviewJob, listJobs } from "@/lib/api";

export default async function ReviewPage() {
  const result = await listJobs({
    status: "reviewing",
    page: 1,
    pageSize: 200,
  });
  const jobs = result.items;
  const initialScrapeData = jobs.length > 0 ? await getReviewJob(jobs[0].id) : null;

  return <ReviewShell jobs={jobs} initialScrapeData={initialScrapeData} />;
}

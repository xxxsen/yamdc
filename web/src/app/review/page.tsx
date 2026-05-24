import { ReviewShell } from "@/components/review-shell";
import { loadReviewInitialData } from "@/lib/server/initial-loaders";

export default async function ReviewPage() {
  const { data, errorMessage } = await loadReviewInitialData();
  return (
    <div style={{ height: "100%" }}>
      <ReviewShell
        jobs={data.jobs}
        initialScrapeData={data.initialScrapeData}
        initialMediaStatus={data.initialMediaStatus}
        initialError={errorMessage}
      />
    </div>
  );
}

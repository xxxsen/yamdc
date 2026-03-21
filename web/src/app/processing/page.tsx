import { JobTable } from "@/components/job-table";
import { listJobs } from "@/lib/api";

export default async function ProcessingPage() {
  const result = await listJobs({
    status: "init,processing,failed,reviewing",
    page: 1,
    pageSize: 20,
  });

  return <JobTable initialData={result} />;
}

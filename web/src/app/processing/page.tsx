import { ProcessingShell } from "@/components/processing-shell";
import { listJobs } from "@/lib/api";

export default async function ProcessingPage() {
  const result = await listJobs({
    status: "init,processing,failed,reviewing",
    all: true,
  });

  return (
    <div style={{ height: "100%" }}>
      <ProcessingShell initialData={result} />
    </div>
  );
}

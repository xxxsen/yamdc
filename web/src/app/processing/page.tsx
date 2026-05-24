import { ProcessingShell } from "@/components/processing-shell";
import { loadProcessingInitialData } from "@/lib/server/initial-loaders";

export default async function ProcessingPage() {
  const { data, errorMessage } = await loadProcessingInitialData();
  return (
    <div style={{ height: "100%" }}>
      <ProcessingShell initialData={data.jobs} initialError={errorMessage} />
    </div>
  );
}

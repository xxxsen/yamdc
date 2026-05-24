import { LibraryShell } from "@/components/library-shell";
import { loadLibraryInitialData } from "@/lib/server/initial-loaders";

export default async function LibraryPage() {
  const { data, errorMessage } = await loadLibraryInitialData();
  return (
    <div style={{ height: "100%" }}>
      <LibraryShell
        items={data.items}
        initialDetail={data.initialDetail}
        initialMediaStatus={data.initialMediaStatus}
        initialError={errorMessage}
      />
    </div>
  );
}

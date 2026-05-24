import { MediaLibraryShell } from "@/components/media-library-shell";
import { loadMediaLibraryInitialData } from "@/lib/server/initial-loaders";

export const dynamic = "force-dynamic";

export default async function MediaLibraryPage() {
  const { data, errorMessage } = await loadMediaLibraryInitialData();
  return (
    <div style={{ height: "100%" }}>
      <MediaLibraryShell
        items={data.items}
        initialStatus={data.initialStatus}
        initialError={errorMessage}
      />
    </div>
  );
}

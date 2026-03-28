import { MediaLibraryShell } from "@/components/media-library-shell";
import type { MediaLibraryItem, MediaLibraryStatus } from "@/lib/api";
import { getMediaLibraryStatus, listMediaLibraryItems } from "@/lib/api";

export default async function MediaLibraryPage() {
  let items: MediaLibraryItem[] = [];
  let initialStatus: MediaLibraryStatus | null = null;
  try {
    items = await listMediaLibraryItems();
  } catch {
    items = [];
  }
  try {
    initialStatus = await getMediaLibraryStatus();
  } catch {
    initialStatus = null;
  }

  return (
    <div style={{ height: "100%" }}>
      <MediaLibraryShell items={items} initialStatus={initialStatus} />
    </div>
  );
}

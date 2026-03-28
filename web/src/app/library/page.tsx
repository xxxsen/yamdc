import { LibraryShell } from "@/components/library-shell";
import { getLibraryItem, getMediaLibraryStatus, listLibraryItems } from "@/lib/api";

export default async function LibraryPage() {
  const items = await listLibraryItems();
  let initialMediaStatus = null;
  let initialDetail = null;
  if (items.length > 0) {
    try {
      initialDetail = await getLibraryItem(items[0].rel_path);
    } catch {
      initialDetail = null;
    }
  }
  try {
    initialMediaStatus = await getMediaLibraryStatus();
  } catch {
    initialMediaStatus = null;
  }

  return (
    <div style={{ height: "100%" }}>
      <LibraryShell items={items} initialDetail={initialDetail} initialMediaStatus={initialMediaStatus} />
    </div>
  );
}

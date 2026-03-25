import { LibraryShell } from "@/components/library-shell";
import { getLibraryItem, listLibraryItems } from "@/lib/api";

export default async function LibraryPage() {
  const items = await listLibraryItems();
  let initialDetail = null;
  if (items.length > 0) {
    try {
      initialDetail = await getLibraryItem(items[0].rel_path);
    } catch {
      initialDetail = null;
    }
  }

  return (
    <div style={{ height: "100%" }}>
      <LibraryShell items={items} initialDetail={initialDetail} />
    </div>
  );
}

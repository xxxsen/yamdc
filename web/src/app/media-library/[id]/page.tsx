import { notFound } from "next/navigation";

import { MediaLibraryDetailShell } from "@/components/media-library-detail-shell";
import { getMediaLibraryItem } from "@/lib/api";

export default async function MediaLibraryDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const itemID = Number.parseInt(id, 10);
  if (!Number.isFinite(itemID) || itemID <= 0) {
    notFound();
  }

  let initialDetail;
  try {
    initialDetail = await getMediaLibraryItem(itemID);
  } catch {
    notFound();
  }

  return (
    <div style={{ height: "100%" }}>
      <MediaLibraryDetailShell initialDetail={initialDetail} />
    </div>
  );
}

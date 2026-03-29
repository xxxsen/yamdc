import { redirect } from "next/navigation";

export default function DebugPage() {
  redirect("/debug/ruleset");
}

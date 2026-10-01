"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function CodexFleetPage() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/fleet/google");
  }, [router]);

  return null;
}

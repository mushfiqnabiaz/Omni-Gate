"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function ClaudeFleetPage() {
  const router = useRouter();
  useEffect(() => {
    router.replace("/fleet/google");
  }, [router]);

  return null;
}

"use client";

import { use } from "react";

import { BundlePage } from "@/components/bundles/BundlePage";

export default function BundleRoute({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return <BundlePage bundleId={decodeURIComponent(id)} />;
}

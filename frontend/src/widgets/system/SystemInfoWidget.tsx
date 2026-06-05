import { useState, useEffect } from "react";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { DescriptionDetails, DescriptionList, DescriptionTerm } from "@/components/ui/description-list";
import { Skeleton } from "@/components/ui/skeleton";
import { useOverview } from "@/hooks/use-overview";
import type { Stats } from "@/types";

export function SystemInfoWidget() {
  const { data: stats } = useOverview((o) => o?.Stats);

  if (!stats) {
    return (
      <Skeleton isLoading>
        <Card className="h-full"><CardContent>{"."}</CardContent></Card>
      </Skeleton>
    );
  }

  return (
    <Card className="h-full">
      <CardHeader title="System Info" />
      <CardContent>
        <DescriptionList>
          <DescriptionTerm>OS</DescriptionTerm>
          <DescriptionDetails>{stats.System.OS}</DescriptionDetails>
          <DescriptionTerm>Kernel</DescriptionTerm>
          <DescriptionDetails>{stats.System.KernelVersion}</DescriptionDetails>
          <DescriptionTerm>CPU</DescriptionTerm>
          <DescriptionDetails>{stats.System.CPUModel}</DescriptionDetails>
          {stats.System.Motherboard && (
            <>
              <DescriptionTerm>Board</DescriptionTerm>
              <DescriptionDetails>{stats.System.Motherboard}</DescriptionDetails>
            </>
          )}
        </DescriptionList>
      </CardContent>
    </Card>
  );
}

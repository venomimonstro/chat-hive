'use client';

import { useEffect } from 'react';
import { GrowthObjectType, recordGrowthEvent } from '../lib/growth';

export function GrowthView({ objectType, objectId }: { objectType: GrowthObjectType; objectId: string }) {
  useEffect(() => {
    void recordGrowthEvent('public_view', objectType, objectId);
  }, [objectType, objectId]);
  return null;
}

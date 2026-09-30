'use client';

import { useEffect } from 'react';
import { RealtimeClient } from '../lib/realtime';

export function RealtimeBridge() {
  useEffect(() => {
    const client = new RealtimeClient();
    client.start();
    return () => client.stop();
  }, []);
  return null;
}

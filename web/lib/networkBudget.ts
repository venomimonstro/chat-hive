export type NetworkHint = { saveData?: boolean; effectiveType?: string } | undefined;
export function messagePageSize(hint: NetworkHint): number {
  if (hint?.saveData || hint?.effectiveType === 'slow-2g' || hint?.effectiveType === '2g') return 24;
  return 40;
}
export function currentMessagePageSize(): number {
  if (typeof navigator === 'undefined') return 40;
  const connection = (navigator as Navigator & {connection?: NetworkHint}).connection;
  return messagePageSize(connection);
}

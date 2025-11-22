/**
 * Formats an ISO date string to "dd.MM.yyyy" format
 * @param isoDate - ISO date string (e.g., "2025-01-18")
 * @returns Formatted date string (e.g., "18.01.2025")
 */
export function formatDate(isoDate: string): string {
  const date = new Date(isoDate);
  const day = String(date.getDate()).padStart(2, '0');
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const year = date.getFullYear();
  return `${day}.${month}.${year}`;
}


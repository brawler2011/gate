/**
 * Formats an ISO date string to "dd.MM.yyyy" format
 * @param isoDate - ISO date string (e.g., "2025-01-18")
 * @returns Formatted date string (e.g., "18.01.2025")
 */
export function formatDate(isoDate: string | undefined): string {
  if (!isoDate) return "—";
  
  try {
    const date = new Date(isoDate);
    
    // Check for Invalid Date
    if (isNaN(date.getTime())) return "—";
    
    // Use UTC methods to avoid timezone issues
    const day = String(date.getUTCDate()).padStart(2, '0');
    const month = String(date.getUTCMonth() + 1).padStart(2, '0');
    const year = date.getUTCFullYear();
    return `${day}.${month}.${year}`;
  } catch {
    return "—";
  }
}


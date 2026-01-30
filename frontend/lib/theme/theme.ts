import { createTheme } from '@mantine/core';

export const theme = createTheme({
  colors: {
    // Custom surface colors for intermediate values
    surface: [
      '#333333', // 0 - same as dark-6
      '#2F2F2F', // 1 - dark-6.5 (between 6 and 7)
      '#2A2A2A', // 2 - same as dark-7
      '#272727', // 3 - same as dark-8
      '#222222', // 4 - same as dark-9
      '#1D1D1D', // 5
      '#181818', // 6
      '#141414', // 7
      '#101010', // 8
      '#0A0A0A', // 9
    ],
    // Pure Neutral Gray Palette (No blue undertones)
    // R=G=B values to ensure it is strictly gray
    dark: [
      '#E1E1E1', // 0 - Text: Primary (Neutral Light Gray)
      '#C7C7C7', // 1 - Text: Secondary
      '#9A9A9A', // 2 - Text: Dimmed
      '#757575', // 3 - Borders: Strong
      '#555555', // 4 - Borders: Subtle / Inputs
      '#424242', // 5 - UI: Hover
      '#333333', // 6 - Surface: Secondary
      '#2A2A2A', // 7 - Surface: Cards (Neutral Gray)
      '#272727', // 8 - Surface: Panels/Active tabs
      '#222222', // 9 - Background: Main Body (Lighter Neutral Gray)
      '#1D1D1D', // 10 - Background: Main Body (Lighter Neutral Gray)
      '#181818', // 11 - Darkest
    ],
  },
  primaryColor: 'blue',
});

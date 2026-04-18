export const getTabStyles = (isActive: boolean) => ({
  root: {
    backgroundColor: isActive
      ? "light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))"
      : "transparent",
    color:
      "light-dark(var(--mantine-color-dark-7), var(--mantine-color-dark-0))",
    border: "1px solid transparent",
    transition: "background-color 120ms ease, color 120ms ease",
    "&:hover": {
      backgroundColor: isActive
        ? "light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))"
        : "transparent",
      color: "light-dark(var(--mantine-color-black), var(--mantine-color-white))",
    },
  },
  section: {
    color: "inherit",
  },
});

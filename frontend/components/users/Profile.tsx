"use client";

import {Container, Stack} from "@mantine/core";

import type {ReactNode} from "react";

type ProfileContainerProps = {
  children: ReactNode;
};

export const ProfileContainer = ({children}: ProfileContainerProps) => {
  return (
    <Container size="md" px={0}>
      <Stack gap="lg">{children}</Stack>
    </Container>
  );
};

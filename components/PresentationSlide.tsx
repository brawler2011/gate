import { 
    Container, 
    Title, 
    Text, 
    Grid, 
    GridCol,
    ThemeIcon, 
    Stack, 
    Group, 
    Paper, 
    Box,
    rem 
  } from '@mantine/core';
  import { 
    IconRocket, 
    IconGavel, 
    IconTools, 
    IconBrandGolang,
    IconBrandNextjs 
  } from '@tabler/icons-react';
  
  export function PresentationSlide() {
    const features = [
      {
        icon: IconRocket,
        title: 'Скорость и Технологии',
        description: 'Next.js 15 (SSR) для мгновенного отклика. Мощный Go-бэкенд для высоких нагрузок.'
      },
      {
        icon: IconGavel,
        title: 'Мгновенная Система Проверки',
        description: 'Автоматический Judge System. Вердикт за миллисекунды. Честное тестирование в изоляции.'
      },
      {
        icon: IconTools,
        title: 'Воркшоп и Сообщество',
        description: 'Инструменты для авторов задач. Создавай контесты, пиши блоги, делись знаниями.'
      },
    ];
  
    return (
      <Box py={80} bg="var(--mantine-color-dark-8)">
        <Container size="xl">
          <Grid gutter={50} align="center">
            {/* Левая колонка: Текст */}
            <GridCol span={{ base: 12, md: 6 }}>
              <Stack gap="xl">
                <div>
                  <Title 
                    order={1} 
                    size={rem(48)} 
                    fw={900} 
                    style={{ lineHeight: 1.1 }}
                  >
                    Gate149
                  </Title>
                  <Text 
                    size="xl" 
                    fw={500} 
                    variant="gradient" 
                    gradient={{ from: 'blue', to: 'cyan', deg: 90 }}
                  >
                    Современная платформа для спортивного программирования
                  </Text>
                </div>
  
                <Stack gap="lg">
                  {features.map((feature, index) => (
                    <Group key={index} wrap="nowrap" align="flex-start">
                      <ThemeIcon 
                        size={48} 
                        radius="md" 
                        variant="gradient" 
                        gradient={{ from: 'blue', to: 'cyan', deg: 135 }}
                      >
                        <feature.icon size={26} stroke={1.5} />
                      </ThemeIcon>
                      <div>
                        <Text size="lg" fw={700} c="white">
                          {feature.title}
                        </Text>
                        <Text c="dimmed" style={{ maxWidth: 400 }}>
                          {feature.description}
                        </Text>
                      </div>
                    </Group>
                  ))}
                </Stack>
  
                <Group mt="xl">
                  <Text size="sm" c="dimmed" tt="uppercase" fw={700} lts={1}>
                    Powered by
                  </Text>
                  <Group gap="xs">
                    <IconBrandNextjs size={24} color="white" />
                    <Text fw={700} c="white">Next.js</Text>
                    <div style={{ width: 1, height: 20, background: '#444' }} />
                    <IconBrandGolang size={24} color="#00ADD8" />
                    <Text fw={700} c="#00ADD8">Go</Text>
                  </Group>
                </Group>
              </Stack>
            </GridCol>
  
            {/* Правая колонка: Визуал (Плейсхолдеры) */}
            <GridCol span={{ base: 12, md: 6 }}>
              <div style={{ position: 'relative' }}>
                {/* Основной скриншот (например, Редактор кода) */}
                <Paper 
                  shadow="xl" 
                  radius="lg" 
                  withBorder 
                  p="xl" 
                  h={300}
                  style={{ 
                    display: 'flex', 
                    alignItems: 'center', 
                    justifyContent: 'center',
                    background: 'var(--mantine-color-dark-7)',
                    borderColor: 'var(--mantine-color-dark-4)',
                    borderStyle: 'dashed'
                  }}
                >
                  <Stack align="center" gap="xs">
                    <IconTools size={48} color="gray" style={{ opacity: 0.3 }} />
                    <Text c="dimmed" ta="center">
                      СКРИНШОТ 1<br/>
                      (Например: Редактор кода с "Accepted")
                    </Text>
                  </Stack>
                </Paper>
  
                {/* Второстепенный скриншот (например, Карточка задачи) */}
                <Paper 
                  shadow="xl" 
                  radius="lg" 
                  withBorder 
                  p="md"
                  h={180}
                  w={280}
                  style={{ 
                    position: 'absolute', 
                    bottom: -40, 
                    left: -40,
                    display: 'flex', 
                    alignItems: 'center', 
                    justifyContent: 'center',
                    background: 'var(--mantine-color-dark-6)',
                    borderColor: 'var(--mantine-color-blue-9)',
                    borderWidth: 2,
                    borderStyle: 'dashed'
                  }}
                >
                   <Stack align="center" gap="xs">
                    <IconGavel size={32} color="var(--mantine-color-blue-5)" style={{ opacity: 0.5 }} />
                    <Text c="dimmed" size="sm" ta="center">
                      СКРИНШОТ 2<br/>
                      (Например: Условие задачи)
                    </Text>
                  </Stack>
                </Paper>
              </div>
            </GridCol>
          </Grid>
        </Container>
      </Box>
    );
  }
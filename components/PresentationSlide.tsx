import { 
    Container, 
    Title, 
    Text, 
    Grid, 
    GridCol,
    ThemeIcon, 
    Stack, 
    Group, 
    AspectRatio, 
    Paper,
    Box,
    Overlay,
    Image,
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
        title: 'Мгновенный Отклик',
        description: 'Сайт летает благодаря Next.js 15 SSR и оптимизированному Go-бэкенду.'
      },
      {
        icon: IconGavel,
        title: 'Автоматическое Тестирование',
        description: 'Поддержка множества языков (C++, Python, Go, Java). Молниеносная проверка решений.'
      },
      {
        icon: IconTools,
        title: 'Воркшоп и Сообщество',
        description: 'Инструменты для авторов задач. Создавай задачи и контесты, делись знаниями.'
      },
    ];

    return (
      <Box 
        pt={60} 
        pb={60} 
        style={{ 
          minHeight: '100vh',
          width: '100%',
          position: 'relative',
          backgroundImage: 'url(/images/presentation-bg.jpg)',
          backgroundSize: 'cover',
          backgroundPosition: 'center',
          overflowY: 'auto',
        }}
      >
          <Overlay color="#000" backgroundOpacity={0.3} zIndex={0} style={{ pointerEvents: 'none' }} />

          {/* Декоративная сетка */}
          <div style={{
            position: 'absolute',
            inset: 0,
            backgroundImage: 'linear-gradient(rgba(255,255,255,0.03) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.03) 1px, transparent 1px)',
            backgroundSize: '40px 40px',
            zIndex: 0,
            pointerEvents: 'none'
          }} />

          <Container size="xl" w="100%" style={{ position: 'relative', zIndex: 1 }}>
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
  
                <Group mt="md">
                  <Text size="md" c="dimmed" tt="uppercase" fw={700} lts={1}>
                    Powered by
                  </Text>
                  <Group gap="md">
                    <IconBrandNextjs size={40} color="white" />
                    <Text size="xl" fw={900} c="white">Next.js</Text>
                    <div style={{ width: 2, height: 32, background: '#444' }} />
                    <IconBrandGolang size={40} color="#00ADD8" />
                  </Group>
                </Group>
              </Stack>
            </GridCol>
  
            {/* Правая колонка: Визуал (Скриншоты) */}
            <GridCol span={{ base: 12, md: 6 }} visibleFrom="md">
              <div style={{ position: 'relative' }}>
                {/* Основной скриншот - code.png */}
                <Paper 
                  shadow="xl" 
                  radius="lg" 
                  withBorder 
                  p={0}
                  h={370}
                  w={800}
                  style={{ 
                    overflow: 'hidden',
                    borderColor: 'var(--mantine-color-dark-4)',
                  }}
                >
                  <Image 
                    src="/images/code.png" 
                    alt="Code Editor Screenshot"
                    fit="cover"
                    h="100%"
                    w="100%"
                  />
                </Paper>
  
                {/* Второстепенный скриншот - verdict.png */}
                <Paper 
                  shadow="xl" 
                  radius="lg" 
                  withBorder 
                  p={0}
                  w={400}
                  h={210}
                  style={{ 
                    position: 'absolute', 
                    bottom: -100, 
                    left: -100,
                    overflow: 'hidden',
                    borderColor: 'var(--mantine-color-dark-4)',
                  }}
                >
                  <Image 
                    src="/images/verdict.png" 
                    alt="Verdict Screenshot"
                    fit="contain"
                    h="100%"
                    w="100%"
                  />
                </Paper>
              </div>
            </GridCol>
          </Grid>
          </Container>
      </Box>
    );
  }
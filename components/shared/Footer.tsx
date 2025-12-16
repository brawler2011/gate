import { Container, Group, Text, Anchor } from '@mantine/core';
import Link from 'next/link';
import classes from './Footer.module.css';

const Footer = () => {
    const currentYear = new Date().getFullYear();

    return (
        <div className={classes.footer}>
            <Container className={classes.container} size="xl">
                <Text c="dimmed" size="sm">
                    © {currentYear} gate149 inc. Все права защищены.
                </Text>

                <Group className={classes.links}>
                    <Anchor component={Link} href="/privacy" c="dimmed" size="sm">
                        Политика конфиденциальности
                    </Anchor>
                </Group>
            </Container>
        </div>
    );
}

export { Footer };

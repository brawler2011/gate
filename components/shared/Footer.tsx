import { Container, Group, Text } from '@mantine/core';
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
                    <Link 
                        href="/privacy" 
                        className={classes.link}
                    >
                        <Text c="dimmed" size="sm">
                            Политика конфиденциальности
                        </Text>
                    </Link>
                </Group>
            </Container>
        </div>
    );
}

export { Footer };

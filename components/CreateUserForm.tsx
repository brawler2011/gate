"use client";

import {Button, Modal, PasswordInput, Stack, Text, TextInput} from "@mantine/core";
import React from "react";
import {useForm} from "@mantine/form";
import {useRouter} from "next/navigation";
import {useMutation} from "@tanstack/react-query";
import {CreationResponseModel} from "@contracts/core/v1";
import {useDisclosure} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconPlus} from "@tabler/icons-react";

type Credentials = {
    username: string,
    password: string
}

type CreateUserFormProps = {
    onSubmitFn: (credentials: Credentials) => Promise<any>
}

const CreateUserForm = (props: CreateUserFormProps) => {
    const [opened, {open, close}] = useDisclosure(false);
    const router = useRouter();

    const form = useForm<Credentials>({
        initialValues: {
            username: "",
            password: ""
        },
    });

    const mutation = useMutation({
        mutationFn: props.onSubmitFn,
        onSuccess: async (data: CreationResponseModel) => {
            // notifications.show({
            //     title: "Успех",
            //     message: "Юзер успешно добавлен",
            //     color: "teal",
            // });
            // close();
            router.push(`/users/${data.id}`)
        },
        onError: (error) => {
            form.clearErrors();
            notifications.show({
                title: "Ошибка",
                message: "Не удалось добавить юзера",
                color: "red",
            })

            // form.setFieldError("username", "Что-то пошло не так. Попробуйте позже.")
        },
        retry: false
    });

    const onSubmit = (event: React.FormEvent<HTMLFormElement>) => {
        if (form.validate().hasErrors) return;

        event.preventDefault()
        mutation.mutate(form.getValues())
    }

    return (
        <>
            <Button
                leftSection={<IconPlus size={16}/>}
                onClick={open}
            >
                Добавить пользователя
            </Button>
            <Modal
                opened={opened}
                onClose={close}
                title={
                    <Text size="lg" fw={500}>
                        Добавление пользователя
                    </Text>
                }
            >
                <form onSubmit={onSubmit} autoComplete="off">
                    <Stack
                        align="center"
                        justify="center"
                        w="fit-content"
                        m="auto"
                        mt="5%"
                        p="md"
                        style={{color: "var(--mantine-color-bright)"}}
                    >
                        <Stack w="100%" gap="0" align="center">
                            <TextInput
                                label="Username"
                                placeholder="Username"
                                autoComplete="off"
                                // key={form.key('username')}
                                w="250"
                                {...form.getInputProps('username')}
                            />
                            <PasswordInput
                                label="Пароль"
                                placeholder="Пароль"
                                autoComplete="off"
                                w="250"
                                // key={form.key('password')}
                                {...form.getInputProps('password')}
                            />
                        </Stack>
                        <Button type="submit" loading={mutation.isPending}>Добавить</Button>
                    </Stack>
                </form>
            </Modal>
        </>
    );
};

export {CreateUserForm};

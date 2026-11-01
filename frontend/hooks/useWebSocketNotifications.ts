import {notifications} from '@mantine/notifications';
import {useEffect, useId, useRef} from 'react';

import type {SubmissionsConnectionStatus} from '@/lib/submissionsConnection';

export const useWebSocketNotifications = (status: SubmissionsConnectionStatus): void => {
  const id = useId();
  const visible = useRef(false);
  const recovering = useRef(false);
  useEffect(() => {
    const show = (title: string, message: string, color: string, autoClose: number | false) => {
      const options = {id, title, message, color, autoClose, onClose: () => {
        visible.current = false;
      }};
      if (visible.current) {
        notifications.update(options);
      } else {
        notifications.show(options);
        visible.current = true;
      }
    };
    if (status.phase === 'idle') {
      notifications.hide(id);
      visible.current = false;
      recovering.current = false;
    } else if (status.phase === 'connected' && recovering.current) {
      show('Подключение восстановлено', 'Онлайн-обновления снова доступны.', 'green', 2500);
      recovering.current = false;
    } else if (status.phase === 'fatal') {
      recovering.current = true;
      show('Онлайн-обновления недоступны',
        'Не удалось восстановить WebSocket после нескольких попыток. Обновите страницу позже.', 'red', false);
    } else if (status.phase === 'reconnecting') {
      const timer = setTimeout(() => {
        recovering.current = true;
        show('Соединение потеряно',
          `Переподключение к обновлениям... попытка ${status.attempt}/${status.maxAttempts}`, 'orange', false);
      }, 400);
      return () => clearTimeout(timer);
    }
  }, [status, id]);
  useEffect(() => () => {
    notifications.hide(id);
  }, [id]);
};

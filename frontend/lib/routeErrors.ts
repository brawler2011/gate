export class RouteRedirect extends Error {
  constructor(public to: string) {
    super(to);
  }
}
export const redirect: (to: string) => never = (to) => {
  throw new RouteRedirect(to);
};
export const notFound: () => never = () => {
  throw {status: 404, message: "Страница не найдена"};
};

import {Component} from "react";
import {Navigate, useLocation} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {RouteRedirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

type Props = { children: ReactNode; resetKey: string };
type State = { error: unknown; resetKey: string };
class Boundary extends Component<Props, State> {
  state: State = {error: null, resetKey: this.props.resetKey};
  static getDerivedStateFromError(error: unknown): Pick<State, "error"> {
    return {error};
  }
  static getDerivedStateFromProps(props: Props, state: State): State | null {
    return props.resetKey === state.resetKey ? null : {error: null, resetKey: props.resetKey};
  }
  render(): ReactNode {
    if (this.state.error instanceof RouteRedirect) {
      return <Navigate to={this.state.error.to} replace />;
    }
    if (this.state.error) {
      const error = this.state.error as { status?: number; message?: string };
      return (
        <ErrorDisplay error={{status: error.status ?? 500, message: error.message ?? "Ошибка загрузки"}} />
      );
    }
    return this.props.children;
  }
}
export const RouteBoundary = ({children}: { children: ReactNode }): ReactNode => {
  const location = useLocation();
  return <Boundary resetKey={location.key}>{children}</Boundary>;
};

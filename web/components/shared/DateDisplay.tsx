import { formatDate, formatDateTime } from "@/lib/utils";

type Props = {
  iso: string;
  withTime?: boolean;
};

export function DateDisplay({ iso, withTime = false }: Props) {
  return (
    <time dateTime={iso} title={iso}>
      {withTime ? formatDateTime(iso) : formatDate(iso)}
    </time>
  );
}

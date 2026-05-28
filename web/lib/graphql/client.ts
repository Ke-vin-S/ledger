import { GraphQLClient } from "graphql-request";
import { getAccessToken } from "../auth";

const GRAPHQL_URL =
  process.env.NEXT_PUBLIC_GRAPHQL_URL ?? "http://localhost:8080/graphql";

export function getGraphQLClient(): GraphQLClient {
  const token = getAccessToken();
  return new GraphQLClient(GRAPHQL_URL, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    credentials: "include",
  });
}

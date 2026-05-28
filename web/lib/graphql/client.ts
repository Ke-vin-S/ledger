import { GraphQLClient } from "graphql-request";
import { getAccessToken } from "../auth";
import { GRAPHQL_URL } from "@/constants/config";

export function getGraphQLClient(): GraphQLClient {
  const token = getAccessToken();
  return new GraphQLClient(GRAPHQL_URL, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    credentials: "include",
  });
}

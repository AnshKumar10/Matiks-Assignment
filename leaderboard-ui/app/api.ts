import axios, { AxiosError } from "axios";
import { LeaderboardUser } from "./types";

const API_BASE = "https://matiks-assignment-production.up.railway.app`";

export const getLeaderboard = async (
  limit: number = 100,
  offset: number = 0,
): Promise<LeaderboardUser[]> => {
  try {
    const res = await axios.get<LeaderboardUser[]>(`${API_BASE}/leaderboard`, {
      params: { limit, offset },
    });

    return res.data;
  } catch (error) {
    const err = error as AxiosError;
    console.error("Error fetching leaderboard:", err.message);
    return [];
  }
};

export const searchUsers = async (
  query: string,
): Promise<LeaderboardUser[]> => {
  try {
    const res = await axios.get<LeaderboardUser[]>(
      `${API_BASE}/leaderboard/users/search`,
      { params: { q: query } },
    );

    return res.data;
  } catch (error) {
    const err = error as AxiosError;
    console.error("Error searching users:", err.message);
    return [];
  }
};

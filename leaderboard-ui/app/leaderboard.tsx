import React, { useEffect, useMemo, useState } from "react";
import { ScrollView, Text, StyleSheet, TextInput, View, RefreshControl } from "react-native";
import { Search } from "lucide-react";
import { getLeaderboard, searchUsers } from "./api";
import { debounce } from "./utils";
import { LeaderboardUser } from "./types";

export default function LeaderboardScreen() {
  const [leaderboard, setLeaderboard] = useState<LeaderboardUser[]>([]);
  const [searchResults, setSearchResults] = useState<LeaderboardUser[]>([]);
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [refreshing, setRefreshing] = useState<boolean>(false);

  const fetchLeaderboard = async (): Promise<void> => {
    setRefreshing(true);
    const data = await getLeaderboard();
    setLeaderboard(data);
    setRefreshing(false);
  };

  useEffect(() => {
    fetchLeaderboard();
    const interval = setInterval(fetchLeaderboard, 15000);

    return () => clearInterval(interval);
  }, []);

  const handleSearch = useMemo(
    () =>
      debounce(async (text) => {
        if (text.length > 0) {
          const results = await searchUsers(text);
          setSearchResults(results);
        } else {
          setSearchResults([]);
        }
      }, 500),
    [],
  );

  const onChangeSearch = (text: string): void => {
    setSearchQuery(text);
    handleSearch(text);
  };

  const displayData = searchQuery.length > 0 ? searchResults : leaderboard;

  return (
    <View style={styles.page}>
      <View style={styles.container}>
        <View style={styles.header}>
          <Text style={styles.title}>LEADERBOARD</Text>
          <Text style={styles.subtitle}>Correctness, Scale and Clarity</Text>
        </View>

        <View style={styles.searchWrapper}>
          <View style={styles.searchIcon}>
            <Search size={20} color="#6b7280" />
          </View>
          <TextInput
            value={searchQuery}
            onChangeText={onChangeSearch}
            placeholder="Search player..."
            placeholderTextColor="#4b5563"
            style={styles.searchInput}
          />
        </View>

        <ScrollView
          style={styles.tableScroll}
          contentContainerStyle={styles.table}
          refreshControl={
            <RefreshControl refreshing={refreshing} onRefresh={fetchLeaderboard} tintColor="#00ff41" />
          }
        >
          {displayData.map((item) => (
            <View key={item.user_id} style={styles.row}>
              <Text
                style={[
                  styles.rank,
                  item.rank === 1 && styles.rankFirst,
                  item.rank === 2 && styles.rankSecond,
                  item.rank === 3 && styles.rankThird,
                ]}
              >
                [{item.rank}]
              </Text>

              <Text
                style={[styles.username, item.rank <= 3 && styles.usernameTop]}
              >
                {item.username}
              </Text>

              <View style={styles.ratingWrapper}>
                <View style={styles.dot} />
                <Text style={styles.rating}>{item.rating}</Text>
              </View>
            </View>
          ))}

          {displayData.length === 0 && (
            <View style={styles.empty}>
              <Text style={styles.emptyText}>No players found</Text>
            </View>
          )}
        </ScrollView>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  page: {
    flex: 1,
    backgroundColor: "#000",
    paddingVertical: 32,
    paddingHorizontal: 16,
    fontFamily: "monospace",
  },
  container: {
    maxWidth: 1100,
    alignSelf: "center",
    width: "100%",
    flex: 1,
  },
  header: {
    alignItems: "center",
    marginBottom: 16,
  },
  title: {
    fontSize: 36,
    fontWeight: "700",
    fontFamily: "monospace",
    color: "#00ff41",
    marginBottom: 8,
    letterSpacing: 2,
  },
  subtitle: {
    color: "#9ca3af",
    fontFamily: "monospace",
    fontSize: 16,
    textAlign: "center",
  },
  searchWrapper: {
    marginBottom: 16,
    maxWidth: 400,
    alignSelf: "center",
    position: "relative",
    width: "100%",
  },
  searchIcon: {
    position: "absolute",
    left: 12,
    top: 12,
    zIndex: 2,
  },
  searchInput: {
    borderWidth: 1,
    borderColor: "#1f2933",
    borderRadius: 8,
    paddingVertical: 12,
    paddingLeft: 40,
    paddingRight: 12,
    color: "#fff",
    backgroundColor: "transparent",
  },
  tableScroll: {
    flex: 1,
    marginTop: 8,
  },
  table: {
    borderTopWidth: 1,
    borderTopColor: "#1f2933",
    paddingRight: 12,
  },
  row: {
    flexDirection: "row",
    alignItems: "center",
    borderBottomWidth: 1,
    borderBottomColor: "#111827",
    paddingVertical: 16,
  },
  rank: {
    width: 60,
    color: "#6b7280",
    fontSize: 13,
    fontFamily: "monospace",
  },
  rankFirst: {
    color: "#00ff41",
    fontWeight: "700",
  },
  rankSecond: {
    color: "#d1d5db",
    fontWeight: "700",
  },
  rankThird: {
    color: "#9ca3af",
    fontWeight: "700",
  },
  username: {
    flex: 1,
    color: "#d1d5db",
    fontFamily: "monospace",
  },
  usernameTop: {
    color: "#ffffff",
  },
  ratingWrapper: {
    flexDirection: "row",
    alignItems: "center",
  },
  rating: {
    color: "#fff",
    fontFamily: "monospace",
    marginRight: 8,
  },
  dot: {
    width: 8,
    height: 8,
    margin: 10,
    borderRadius: 4,
    backgroundColor: "#00d4ff",
  },
  empty: {
    paddingVertical: 64,
    alignItems: "center",
  },
  emptyText: {
    color: "#6b7280",
    fontFamily: "monospace",
  },
});
